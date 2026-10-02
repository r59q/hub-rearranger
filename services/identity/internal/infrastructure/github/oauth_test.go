package github

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/r59q/hub-rearranger/services/identity/internal/domain"
)

// All token-like strings in these tests are synthetic, never live credentials.
func TestOAuthExchangeAndRefreshUseStandardPKCEAndRotate(t *testing.T) {
	for _, encoding := range []string{"json", "form"} {
		t.Run(encoding, func(t *testing.T) {
			requests := 0
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				_ = r.ParseForm()
				if r.Method != "POST" || r.Form.Get("client_id") != "Iv1.synthetic" || r.Form.Get("client_secret") != "synthetic-secret" {
					t.Error("OAuth client authentication missing")
				}
				refresh := r.Form.Get("grant_type") == "refresh_token"
				if !refresh && (r.Form.Get("code_verifier") != strings.Repeat("v", 43) || r.Form.Get("redirect_uri") != "http://localhost:3000/auth/callback" || r.Form.Get("code") != "synthetic-code") {
					t.Error("PKCE callback policy missing")
				}
				if refresh && r.Form.Get("refresh_token") != "ghr_synthetic-one" {
					t.Error("wrong refresh token")
				}
				access, refreshToken := "ghu_synthetic-one", "ghr_synthetic-one"
				if refresh {
					access, refreshToken = "ghu_synthetic-two", "ghr_synthetic-two"
				}
				if encoding == "form" {
					w.Header().Set("Content-Type", "application/x-www-form-urlencoded")
					_, _ = fmt.Fprintf(w, "access_token=%s&refresh_token=%s&token_type=bearer&expires_in=28800&refresh_token_expires_in=15897600&scope=", access, refreshToken)
				} else {
					w.Header().Set("Content-Type", "application/json")
					_ = json.NewEncoder(w).Encode(map[string]any{"access_token": access, "refresh_token": refreshToken, "token_type": "bearer", "expires_in": 28800, "refresh_token_expires_in": 15897600, "scope": ""})
				}
			}))
			defer upstream.Close()
			provider := New(1, "Iv1.synthetic", "synthetic-secret", "http://localhost:3000/auth/callback", upstream.Client())
			provider.oauth.Endpoint.TokenURL = upstream.URL
			verifier := strings.Repeat("v", 43)

			authorization, _ := url.Parse(provider.AuthorizationURL("synthetic-state", verifier))
			sum := sha256.Sum256([]byte(verifier))
			if authorization.Query().Get("code_challenge") != base64.RawURLEncoding.EncodeToString(sum[:]) || authorization.Query().Get("code_challenge_method") != "S256" || authorization.Query().Get("scope") != "" {
				t.Fatal("OAuth authorization lacks PKCE or requests broad scopes")
			}

			first, err := provider.Exchange(context.Background(), "synthetic-code", verifier)

			if err != nil || first.Access != "ghu_synthetic-one" || !first.RefreshExpires.After(time.Now()) {
				t.Fatal("token exchange failed")
			}

			second, err := provider.Refresh(context.Background(), first)

			if err != nil || second.Access != "ghu_synthetic-two" || second.Refresh == first.Refresh || requests != 2 {
				t.Fatal("refresh did not rotate the pair")
			}
		})
	}
}

func TestOAuthRejectsOtherTokenTypesAndSafeErrors(t *testing.T) {
	for _, token := range []string{"ghp_synthetic", "ghs_synthetic", "gho_synthetic"} {
		t.Run(token, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = fmt.Fprintf(w, `{"access_token":%q,"token_type":"bearer"}`, token)
			}))
			defer upstream.Close()
			provider := New(1, "Iv1.synthetic", "synthetic-secret", "http://localhost:3000/auth/callback", upstream.Client())
			provider.oauth.Endpoint.TokenURL = upstream.URL

			_, err := provider.Exchange(context.Background(), "synthetic-code", "verifier")

			if !errors.Is(err, domain.ErrReconnect) {
				t.Fatal("non-user token accepted")
			}
		})
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"error":"invalid_grant","error_description":"private-sentinel"}`))
	}))
	defer upstream.Close()
	provider := New(1, "Iv1.synthetic", "synthetic-secret", "http://localhost:3000/auth/callback", upstream.Client())
	provider.oauth.Endpoint.TokenURL = upstream.URL

	_, err := provider.Exchange(context.Background(), "synthetic-code", "verifier")

	if !errors.Is(err, domain.ErrReconnect) || strings.Contains(err.Error(), "private-sentinel") {
		t.Fatal("unsafe OAuth error")
	}
}
