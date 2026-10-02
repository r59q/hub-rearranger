package api

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/r59q/hub-rearranger/services/identity/internal/domain"
)

func TestSignInAndAuthorizationUseVerifiedUserAndSecureCookies(t *testing.T) {
	handler, provider := setup(t)
	session, csrf := signIn(t, handler, provider)
	if !session.HttpOnly || !session.Secure || session.SameSite != http.SameSiteLaxMode || session.Path != "/" || session.Domain != "" || len(session.Value) != 43 {
		t.Fatal("unsafe session cookie")
	}

	request, response := call(handler, "POST", "/v1/repositories/octo/demo/authorization", fmt.Sprintf(`{"csrf":%q,"action":"assign_comment"}`, csrf), session)
	if response.Code != 200 || !strings.Contains(response.Body.String(), `"login":"octocat"`) {
		t.Fatal("verified identity missing")
	}
	assertContract(t, request, response)

	provider.access.Role = "write"

	request, response = call(handler, "POST", "/v1/repositories/octo/demo/authorization", fmt.Sprintf(`{"csrf":%q,"action":"assign_comment"}`, csrf), session)
	if response.Code != 403 {
		t.Fatal("role revocation was ignored")
	}
	assertContract(t, request, response)
}

func TestCallbackCannotBeForgedOrReplayed(t *testing.T) {
	handler, provider := setup(t)
	_, start := call(handler, "POST", "/v1/sign-in", "")
	binding := cookie(start, "__Host-hub_login")
	path := "/v1/sign-in/callback?state=" + provider.state + "&code=synthetic-code"

	request, response := call(handler, "GET", path, "")
	if response.Code != 403 || provider.exchanges != 0 {
		t.Fatal("missing browser cookie reached exchange")
	}
	assertContract(t, request, response)

	request, response = call(handler, "GET", path, "", binding)
	if response.Code != 303 {
		t.Fatal("valid browser could not sign in")
	}
	assertContract(t, request, response)

	request, response = call(handler, "GET", path, "", binding)
	if response.Code != 403 || provider.exchanges != 1 {
		t.Fatal("replayed callback reached exchange")
	}
	assertContract(t, request, response)
}

func TestForgeryAndMalformedRequestsFailBeforeGitHub(t *testing.T) {
	for _, variant := range []string{"origin", "missing-origin", "csrf", "body", "action", "repository", "duplicate-cookie"} {
		t.Run(variant, func(t *testing.T) {
			handler, provider := setup(t)
			session, csrf := signIn(t, handler, provider)
			checks := provider.userChecks
			body := fmt.Sprintf(`{"csrf":%q,"action":"assign_comment"}`, csrf)
			path := "/v1/repositories/octo/demo/authorization"
			switch variant {
			case "csrf":
				body = `{"csrf":"forged","action":"assign_comment"}`
			case "body":
				body += `{}`
			case "action":
				body = fmt.Sprintf(`{"csrf":%q,"action":"merge"}`, csrf)
			case "repository":
				path = "/v1/repositories/octo/demo%20bad/authorization"
			}
			request := httptest.NewRequest("POST", path, strings.NewReader(body))
			request.Header.Set("Origin", origin)
			request.Header.Set("Content-Type", "application/json")
			request.AddCookie(session)
			if variant == "origin" {
				request.Header.Set("Origin", "https://attacker.example")
			}
			if variant == "missing-origin" {
				request.Header.Del("Origin")
			}
			if variant == "duplicate-cookie" {
				request.AddCookie(session)
			}
			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)

			if response.Code < 400 || provider.userChecks != checks {
				t.Fatal("untrusted request accepted")
			}
		})
	}
}

func TestRevokedSessionIsClearedAndProviderErrorsAreSafe(t *testing.T) {
	for _, failure := range []error{domain.ErrReconnect, errors.New("private-sentinel")} {
		t.Run(failure.Error(), func(t *testing.T) {
			handler, provider := setup(t)
			session, _ := signIn(t, handler, provider)
			provider.failure = failure

			request, response := call(handler, "GET", "/v1/session", "", session)
			assertContract(t, request, response)
			if failure == domain.ErrReconnect && (response.Code != 401 || cookie(response, "__Host-hub_session").MaxAge != -1) {
				t.Fatal("revoked cookie remained active")
			}
			if failure != domain.ErrReconnect && response.Code != 503 {
				t.Fatal("temporary failure claimed a session")
			}
		})
	}
}

func TestSignOutRemovesLocalSessionDespiteRevocationFailure(t *testing.T) {
	handler, provider := setup(t)
	session, csrf := signIn(t, handler, provider)
	provider.revokeFailure = domain.ErrUnavailable

	request, response := call(handler, "POST", "/v1/sign-out", fmt.Sprintf(`{"csrf":%q}`, csrf), session)
	if response.Code != 200 || !strings.Contains(response.Body.String(), `"revoked":false`) || cookie(response, "__Host-hub_session").MaxAge != -1 {
		t.Fatal("sign-out failed to remove authority")
	}
	assertContract(t, request, response)

	_, response = call(handler, "GET", "/v1/session", "", session)
	if response.Code != 401 {
		t.Fatal("signed-out session was reusable")
	}
}

func TestDisabledServicePreservesReadOnlyStartup(t *testing.T) {
	handler := NewHandler(domain.NewService(nil, nil, nil), origin, true)
	for _, path := range []string{"/health", "/v1/session"} {
		request, response := call(handler, "GET", path, "")
		if response.Code != 200 {
			t.Fatal("unconfigured identity failed read status")
		}
		assertContract(t, request, response)
	}

	request, response := call(handler, "POST", "/v1/sign-in", "")
	if response.Code != 503 {
		t.Fatal("unconfigured sign-in was accepted")
	}
	assertContract(t, request, response)
}
