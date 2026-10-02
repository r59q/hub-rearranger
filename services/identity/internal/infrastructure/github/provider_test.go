package github

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/r59q/hub-rearranger/services/identity/internal/domain"
)

type boundary struct {
	role         string
	appID        int64
	repositoryID int64
	permission   string
	suspended    bool
	revoked      bool
	requests     int
	writes       int
}

func fixture(t *testing.T) (*Provider, *boundary) {
	t.Helper()
	state := &boundary{role: "maintain", appID: 9, repositoryID: 42, permission: "write"}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		state.requests++
		w.Header().Set("Content-Type", "application/json")
		var value any
		if r.URL.Path == "/applications/Iv1.synthetic/token" {
			user, password, ok := r.BasicAuth()
			if !ok || user != "Iv1.synthetic" || password != "synthetic-secret" {
				t.Error("App-bound check lacks Basic authentication")
			}
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["access_token"] != "ghu_synthetic-user" {
				t.Error("wrong user token checked")
			}
			if state.revoked {
				w.WriteHeader(404)
				_, _ = w.Write([]byte(`{"message":"private-sentinel"}`))
				return
			}
			if r.Method == "DELETE" {
				w.WriteHeader(204)
				return
			}
			value = map[string]any{"app": map[string]string{"client_id": "Iv1.synthetic"}, "user": map[string]any{"id": 7}}
		} else {
			if r.Header.Get("Authorization") != "Bearer ghu_synthetic-user" {
				t.Error("REST request did not use user identity")
			}
			switch r.URL.Path {
			case "/user":
				value = map[string]any{"id": 7, "login": "octocat", "type": "User"}
			case "/repos/octo/demo/issues/1/comments":
				if r.Method != "POST" {
					t.Error("comment write used the wrong method")
				}
				state.writes++
				w.WriteHeader(http.StatusCreated)
				value = map[string]any{"id": 55, "user": map[string]any{"id": 7, "login": "octocat"}}
			case "/repos/octo/demo":
				value = map[string]any{"id": 42, "full_name": "octo/demo"}
			case "/repos/octo/demo/collaborators/octocat/permission":
				value = map[string]any{"permission": "write", "role_name": state.role, "user": map[string]any{"id": 7}}
			case "/user/installations":
				installation := map[string]any{"id": 99, "app_id": state.appID, "permissions": map[string]string{"issues": state.permission, "contents": state.permission, "pull_requests": state.permission, "workflows": state.permission}}
				if state.suspended {
					installation["suspended_at"] = "2026-01-01T00:00:00Z"
				}
				value = map[string]any{"total_count": 1, "installations": []any{installation}}
			case "/user/installations/99/repositories":
				value = map[string]any{"total_count": 1, "repositories": []any{map[string]any{"id": state.repositoryID}}}
			default:
				t.Errorf("unexpected GitHub path %s", r.URL.Path)
				w.WriteHeader(404)
				return
			}
		}
		_ = json.NewEncoder(w).Encode(value)
	}))
	t.Cleanup(server.Close)
	provider := New(9, "Iv1.synthetic", "synthetic-secret", "http://localhost:3000/auth/callback", server.Client())
	provider.api.BaseURL, _ = provider.api.BaseURL.Parse(server.URL + "/")
	provider.app.BaseURL, _ = provider.app.BaseURL.Parse(server.URL + "/")
	return provider, state
}

func TestAppBoundIdentityAndCurrentInstallationPermissions(t *testing.T) {
	provider, _ := fixture(t)

	user, err := provider.User(context.Background(), "ghu_synthetic-user")

	if err != nil || user.ID != 7 || user.Login != "octocat" {
		t.Fatal("identity verification failed")
	}

	access, err := provider.RepositoryAccess(context.Background(), "ghu_synthetic-user", user, domain.Repository{Owner: "octo", Name: "demo"})

	if err != nil || access.Role != "maintain" || !access.IssuesWrite || !access.WorkflowsWrite {
		t.Fatal("exact role or installed permission was lost")
	}

	if err := provider.Revoke(context.Background(), "ghu_synthetic-user"); err != nil {
		t.Fatal(err)
	}
}

func TestPublicRepositoryDoesNotBypassInstallationSelection(t *testing.T) {
	for _, variant := range []string{"other-app", "unselected", "suspended"} {
		t.Run(variant, func(t *testing.T) {
			provider, state := fixture(t)
			switch variant {
			case "other-app":
				state.appID = 10
			case "unselected":
				state.repositoryID = 100
			case "suspended":
				state.suspended = true
			}

			_, err := provider.RepositoryAccess(context.Background(), "ghu_synthetic-user", domain.User{ID: 7, Login: "octocat"}, domain.Repository{Owner: "octo", Name: "demo"})

			if !errors.Is(err, domain.ErrForbidden) {
				t.Fatal("public visibility granted write authority")
			}
		})
	}
}

func TestReadOnlyPermissionsAndRevokedTokensRemainUnusable(t *testing.T) {
	provider, state := fixture(t)
	state.permission = "read"

	access, err := provider.RepositoryAccess(context.Background(), "ghu_synthetic-user", domain.User{ID: 7, Login: "octocat"}, domain.Repository{Owner: "octo", Name: "demo"})

	if err != nil || access.IssuesWrite || access.ContentsWrite || access.WorkflowsWrite {
		t.Fatal("read-only grant became write authority")
	}
	state.revoked = true

	_, err = provider.User(context.Background(), "ghu_synthetic-user")

	if !errors.Is(err, domain.ErrReconnect) {
		t.Fatal("revoked user token was accepted")
	}
}

func TestDiscoveryTokenNeverReachesProvider(t *testing.T) {
	provider, state := fixture(t)

	_, err := provider.User(context.Background(), "ghp_synthetic-discovery")

	if !errors.Is(err, domain.ErrReconnect) || state.requests != 0 {
		t.Fatal("discovery token was used as identity")
	}
}
