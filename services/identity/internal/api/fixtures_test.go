package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers/legacy"
	"github.com/r59q/hub-rearranger/services/identity/internal/domain"
	"github.com/r59q/hub-rearranger/services/identity/internal/infrastructure/sqlite"
)

const origin = "https://hub.example.com"

type testProvider struct {
	state         string
	exchanges     int
	userChecks    int
	failure       error
	access        domain.Access
	revokeFailure error
}

func (p *testProvider) AuthorizationURL(state, verifier string) string {
	p.state = state
	return "https://github.com/login/oauth/authorize?state=" + state
}

func (p *testProvider) Exchange(context.Context, string, string) (domain.Credentials, error) {
	p.exchanges++
	return domain.Credentials{Access: "synthetic-access-never-live", Refresh: "synthetic-refresh-never-live"}, nil
}

func (p *testProvider) Refresh(context.Context, domain.Credentials) (domain.Credentials, error) {
	return domain.Credentials{}, domain.ErrReconnect
}

func (p *testProvider) User(context.Context, string) (domain.User, error) {
	p.userChecks++
	return domain.User{ID: 7, Login: "octocat"}, p.failure
}

func (p *testProvider) RepositoryAccess(context.Context, string, domain.User, domain.Repository) (domain.Access, error) {
	return p.access, nil
}

func (p *testProvider) Revoke(context.Context, string) error { return p.revokeFailure }

func setup(t *testing.T) (http.Handler, *testProvider) {
	t.Helper()
	store, err := sqlite.Open(filepath.Join(t.TempDir(), "identity.db"), bytes.Repeat([]byte{9}, 32))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	provider := &testProvider{access: domain.Access{RepositoryID: 42, FullName: "octo/demo", Role: "maintain", IssuesWrite: true, ContentsWrite: true, PullRequestsWrite: true, WorkflowsWrite: true}}
	return NewHandler(domain.NewService(provider, store, time.Now), origin, true), provider
}

func call(handler http.Handler, method, path, body string, cookies ...*http.Cookie) (*http.Request, *httptest.ResponseRecorder) {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Origin", origin)
	request.Header.Set("Content-Type", "application/json")
	for _, cookie := range cookies {
		request.AddCookie(cookie)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return request, response
}

func cookie(response *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, value := range response.Result().Cookies() {
		if value.Name == name {
			return value
		}
	}
	return nil
}

func signIn(t *testing.T, handler http.Handler, provider *testProvider) (*http.Cookie, string) {
	t.Helper()
	_, start := call(handler, "POST", "/v1/sign-in", "")
	binding := cookie(start, "__Host-hub_login")
	if binding == nil {
		t.Fatal("binding cookie missing")
	}
	request, callback := call(handler, "GET", "/v1/sign-in/callback?state="+provider.state+"&code=synthetic-code", "", binding)
	if callback.Code != 303 {
		t.Fatal("callback failed")
	}
	assertContract(t, request, callback)
	session := cookie(callback, "__Host-hub_session")
	if session == nil {
		t.Fatal("session cookie missing")
	}
	request, status := call(handler, "GET", "/v1/session", "", session)
	assertContract(t, request, status)
	var identity struct {
		CSRF string `json:"csrf"`
	}
	_ = json.Unmarshal(status.Body.Bytes(), &identity)
	return session, identity.CSRF
}

func assertContract(t *testing.T, request *http.Request, response *httptest.ResponseRecorder) {
	t.Helper()
	document, err := openapi3.NewLoader().LoadFromFile("../../api/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if err := document.Validate(context.Background()); err != nil {
		t.Fatal(err)
	}
	document.Servers = nil
	router, err := legacy.NewRouter(document)
	if err != nil {
		t.Fatal(err)
	}
	route, params, err := router.FindRoute(request)
	if err != nil {
		t.Fatal(err)
	}
	input := &openapi3filter.RequestValidationInput{Request: request, PathParams: params, Route: route}
	output := &openapi3filter.ResponseValidationInput{RequestValidationInput: input, Status: response.Code, Header: response.Header(), Options: &openapi3filter.Options{IncludeResponseStatus: true}}
	output.SetBodyBytes(response.Body.Bytes())
	if err := openapi3filter.ValidateResponse(context.Background(), output); err != nil {
		t.Fatal(err)
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("identity response was cacheable")
	}
	for _, secret := range []string{"synthetic-access-never-live", "synthetic-refresh-never-live", "private-sentinel"} {
		if strings.Contains(response.Body.String(), secret) {
			t.Fatal("secret or raw error leaked")
		}
	}
}
