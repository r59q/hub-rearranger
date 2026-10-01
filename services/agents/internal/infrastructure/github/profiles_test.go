package github

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	gh "github.com/google/go-github/v74/github"

	"github.com/r59q/hub-rearranger/services/agents/internal/domain"
	"github.com/r59q/hub-rearranger/services/agents/internal/infrastructure/profiles"
)

const testSHA = "0123456789abcdef0123456789abcdef01234567"

func TestReadPinsCatalogToDefaultBranchCommit(t *testing.T) {
	// Arrange.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer synthetic-test-token" {
			t.Error("server-side token was not used")
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/repos/octo/demo":
			_ = json.NewEncoder(w).Encode(map[string]any{"default_branch": "trunk"})
		case "/repos/octo/demo/branches/trunk":
			_ = json.NewEncoder(w).Encode(map[string]any{"commit": map[string]any{"sha": testSHA}})
		case "/repos/octo/demo/contents/.github/agent-profiles.yml":
			if r.URL.Query().Get("ref") != testSHA {
				t.Error("read used a mutable ref")
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"type": "file", "encoding": "base64", "size": 4, "sha": strings.Repeat("f", 40), "content": base64.StdEncoding.EncodeToString([]byte("test"))})
		default:
			t.Errorf("unexpected request: %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client := gh.NewClient(server.Client()).WithAuthToken("synthetic-test-token")
	client.BaseURL, _ = client.BaseURL.Parse(server.URL + "/")
	reader := NewProfileReader(client)
	// Act.
	file, err := reader.ReadCatalog(context.Background(), domain.Repository{Owner: "octo", Name: "demo"})
	// Assert.
	if err != nil || file.Problem != nil || file.DefaultBranch != "trunk" || file.Revision != testSHA || string(file.Content) != "test" {
		t.Fatalf("file = %v, error = %v", file, err)
	}
}

func TestCatalogFileStatesAndGitHubFailures(t *testing.T) {
	cases := []struct {
		name, failPath string
		status         int
		body           any
		wantCode       string
		wantErr        error
	}{
		{"missing", "contents/.github/agent-profiles.yml", 404, nil, "MISSING_FILE", nil},
		{"directory", "contents/.github/agent-profiles.yml", 200, []any{}, "INVALID_FILE", nil},
		{"symlink", "contents/.github/agent-profiles.yml", 200, map[string]any{"type": "symlink", "target": "secret"}, "INVALID_FILE", nil},
		{"submodule", "contents/.github/agent-profiles.yml", 200, map[string]any{"type": "file", "submodule_git_url": "secret"}, "INVALID_FILE", nil},
		{"oversized", "contents/.github/agent-profiles.yml", 200, map[string]any{"type": "file", "size": profiles.MaxBytes + 1}, "LIMIT_EXCEEDED", nil},
		{"bad encoding", "contents/.github/agent-profiles.yml", 200, map[string]any{"type": "file", "encoding": "base64", "content": "%%%"}, "", domain.ErrGitHubUnavailable},
		{"encoding absent", "contents/.github/agent-profiles.yml", 200, map[string]any{"type": "file", "content": "private-sentinel"}, "", domain.ErrGitHubUnavailable},
		{"repository not visible", "", 404, nil, "", domain.ErrRepositoryUnavailable},
		{"branch missing", "branches/main", 404, nil, "", domain.ErrRepositoryUnavailable},
		{"access denied", "contents/.github/agent-profiles.yml", 403, nil, "", domain.ErrAccessDenied},
		{"token invalid", "", 401, nil, "", domain.ErrAccessDenied},
		{"rate limited", "", 429, nil, "", domain.ErrRateLimited},
		{"upstream failure", "", 500, nil, "", domain.ErrGitHubUnavailable},
		{"invalid commit", "branches/main", 200, map[string]any{"commit": map[string]any{"sha": "private-sentinel"}}, "", domain.ErrGitHubUnavailable},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			// Arrange.
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Path == "/repos/octo/demo"+func() string {
					if item.failPath == "" {
						return ""
					}
					return "/" + item.failPath
				}() {
					w.WriteHeader(item.status)
					body := item.body
					if body == nil {
						body = map[string]any{"message": "private-sentinel"}
					}
					_ = json.NewEncoder(w).Encode(body)
					return
				}
				if strings.HasSuffix(r.URL.Path, "/branches/main") {
					_ = json.NewEncoder(w).Encode(map[string]any{"commit": map[string]any{"sha": testSHA}})
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"default_branch": "main"})
			}))
			defer server.Close()
			client := gh.NewClient(server.Client())
			client.BaseURL, _ = client.BaseURL.Parse(server.URL + "/")
			// Act.
			file, err := NewProfileReader(client).ReadCatalog(context.Background(), domain.Repository{Owner: "octo", Name: "demo"})
			// Assert.
			if !errors.Is(err, item.wantErr) {
				t.Fatalf("error = %v, want = %v", err, item.wantErr)
			}
			if item.wantCode != "" && (file.Problem == nil || file.Problem.Code != item.wantCode) {
				t.Fatalf("file = %v", file)
			}
			if item.wantErr != nil && strings.Contains(err.Error(), "private-sentinel") {
				t.Fatal("upstream detail escaped adapter")
			}
		})
	}
}

func TestRateLimitHeadersAreRecognizedOnBranchLookup(t *testing.T) {
	for _, header := range []string{"X-RateLimit-Remaining", "Retry-After"} {
		t.Run(header, func(t *testing.T) {
			// Arrange.
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if strings.HasSuffix(r.URL.Path, "/branches/main") {
					value := "0"
					if header == "Retry-After" {
						value = "60"
					}
					w.Header().Set(header, value)
					w.WriteHeader(http.StatusForbidden)
					_ = json.NewEncoder(w).Encode(map[string]any{"message": "private-sentinel"})
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"default_branch": "main"})
			}))
			defer server.Close()
			client := gh.NewClient(server.Client())
			client.BaseURL, _ = client.BaseURL.Parse(server.URL + "/")
			// Act.
			_, err := NewProfileReader(client).ReadCatalog(context.Background(), domain.Repository{Owner: "octo", Name: "demo"})
			// Assert.
			if !errors.Is(err, domain.ErrRateLimited) {
				t.Fatalf("error = %v", err)
			}
		})
	}
}
