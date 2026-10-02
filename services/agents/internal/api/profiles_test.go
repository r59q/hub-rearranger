package api

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	gh "github.com/google/go-github/v74/github"

	"github.com/r59q/hub-rearranger/services/agents/internal/api/contract"
	"github.com/r59q/hub-rearranger/services/agents/internal/domain"
	githubinfra "github.com/r59q/hub-rearranger/services/agents/internal/infrastructure/github"
	"github.com/r59q/hub-rearranger/services/agents/internal/infrastructure/profiles"
)

const profileSHA = "0123456789abcdef0123456789abcdef01234567"
const profilePath = "/v1/repositories/octo/demo/profiles"

func TestRepositoryProfileReadIntegrationHonorsContract(t *testing.T) {
	valid, err := os.ReadFile("../../../../.github/agent-profiles.yml")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name    string
		content []byte
		missing bool
		state   contract.RepositoryProfilesState
	}{
		{"valid codex-thorough", valid, false, contract.RepositoryProfilesStateValid},
		{"malformed YAML", []byte("private-sentinel: ["), false, contract.RepositoryProfilesStateInvalid},
		{"invalid schema", []byte("schema_version: 1\nprofiles: {private-sentinel: {}}"), false, contract.RepositoryProfilesStateInvalid},
		{"unsupported schema", []byte("schema_version: 2\nprofiles: {}"), false, contract.RepositoryProfilesStateUnsupported},
		{"missing catalog", nil, true, contract.RepositoryProfilesStateMissing},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			// Arrange. The controlled GitHub boundary holds only public configuration.
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/repos/octo/demo":
					_ = json.NewEncoder(w).Encode(map[string]any{"default_branch": "main"})
				case "/repos/octo/demo/branches/main":
					_ = json.NewEncoder(w).Encode(map[string]any{"commit": map[string]any{"sha": profileSHA}})
				case "/repos/octo/demo/contents/.github/agent-profiles.yml":
					if r.URL.Query().Get("ref") != profileSHA {
						t.Error("catalog was not pinned")
					}
					if item.missing {
						http.NotFound(w, r)
						return
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"type": "file", "encoding": "base64", "size": len(item.content), "content": base64.StdEncoding.EncodeToString(item.content)})
				default:
					t.Errorf("unexpected GitHub path: %s", r.URL.Path)
				}
			}))
			defer upstream.Close()
			client := gh.NewClient(upstream.Client())

			client.BaseURL, _ = client.BaseURL.Parse(upstream.URL + "/")
			validator, err := profiles.NewValidator()
			if err != nil {
				t.Fatal(err)
			}
			handler := NewHandler(domain.NewService(domain.NewProfileService(githubinfra.NewProfileReader(client), validator)), slog.New(slog.NewTextHandler(io.Discard, nil)))
			for _, method := range []string{http.MethodGet, http.MethodHead} {
				request := httptest.NewRequest(method, profilePath, nil)
				response := httptest.NewRecorder()

				// Act.
				handler.ServeHTTP(response, request)

				// Assert.
				if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "no-store" {
					t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
				}
				assertContractResponse(t, request, response)
				if method == http.MethodHead {
					if response.Body.Len() != 0 {
						t.Fatal("HEAD returned a body")
					}
					continue
				}
				var result contract.RepositoryProfiles
				if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
					t.Fatal(err)
				}
				if result.State != item.state || result.Revision != profileSHA {
					t.Fatalf("result = %v", result)
				}
				if item.state == contract.RepositoryProfilesStateValid {
					if len(result.Profiles) != 1 || result.Profiles[0].Id != "codex-thorough" || result.Profiles[0].Revision != profileSHA {
						t.Fatalf("profiles = %v", result.Profiles)
					}
				} else if len(result.Profiles) != 0 || len(result.Diagnostics) == 0 || strings.Contains(response.Body.String(), "private-sentinel") {
					t.Fatalf("unsafe or partial result: %s", response.Body.String())
				}
			}
		})
	}
}

type profileFailureService struct {
	err    error
	called bool
}

func (s *profileFailureService) AssignmentConvention(ctx context.Context) (domain.Convention, error) {
	return domain.NewService(nil).AssignmentConvention(ctx)
}

func (s *profileFailureService) RepositoryProfiles(_ context.Context, _ domain.Repository) (domain.ProfileCatalog, error) {
	s.called = true
	return domain.ProfileCatalog{}, s.err
}

func TestProfileFailureStatusesAndLogsAreSafe(t *testing.T) {
	for err, status := range map[error]int{domain.ErrAccessDenied: 403, domain.ErrRepositoryUnavailable: 404, domain.ErrRateLimited: 429, domain.ErrGitHubUnavailable: 502, context.Canceled: 502, context.DeadlineExceeded: 502, errors.New("private-sentinel"): 500} {
		t.Run(err.Error(), func(t *testing.T) {
			// Arrange.
			service := &profileFailureService{err: err}
			var logs bytes.Buffer
			handler := NewHandler(service, slog.New(slog.NewTextHandler(&logs, nil)))
			request := httptest.NewRequest(http.MethodGet, profilePath, nil)
			response := httptest.NewRecorder()

			// Act.
			handler.ServeHTTP(response, request)

			// Assert.
			if response.Code != status || !service.called {
				t.Fatalf("status = %d", response.Code)
			}
			if strings.Contains(response.Body.String()+logs.String(), "private-sentinel") {
				t.Fatal("private error leaked")
			}
			assertContractResponse(t, request, response)
		})
	}
}

func TestProfileInputAndWriteMethodsDoNotCallDomain(t *testing.T) {
	for _, item := range []struct {
		method, path string
		status       int
	}{
		{http.MethodGet, "/v1/repositories/invalid_owner/demo/profiles", 400},
		{http.MethodGet, "/v1/repositories/octo/%3F/profiles", 400},
		{http.MethodPost, profilePath, 405},
		{http.MethodDelete, profilePath, 405},
	} {
		t.Run(item.method+item.path, func(t *testing.T) {
			// Arrange.
			service := &profileFailureService{err: errors.New("private-sentinel")}
			handler := NewHandler(service, slog.New(slog.NewTextHandler(io.Discard, nil)))
			response := httptest.NewRecorder()

			// Act.
			handler.ServeHTTP(response, httptest.NewRequest(item.method, item.path, nil))

			// Assert.
			if response.Code != item.status || service.called {
				t.Fatalf("status = %d, called = %t", response.Code, service.called)
			}
		})
	}
}

func (s *profileFailureService) RepositoryReadiness(ctx context.Context, repo domain.Repository) (domain.RepositoryReadiness, error) {
	_, err := s.RepositoryProfiles(ctx, repo)
	return domain.RepositoryReadiness{}, err
}
