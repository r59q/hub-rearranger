package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/r59q/hub-rearranger/services/identity/internal/domain"
)

type apiBootstrap struct {
	called int
	draft  *domain.ProfileDraft
	err    error
}

func (p *apiBootstrap) Proposal(_ context.Context, repo domain.Repository, review domain.BootstrapReview) (domain.BootstrapProposal, error) {
	p.draft = review.Draft
	return domain.BootstrapProposal{Repository: repo.Owner + "/" + repo.Name, BaseRevision: review.BaseRevision, Digest: review.Digest, DefaultBranch: "main", Changes: []domain.BootstrapChange{{Path: "AGENTS.md", Content: "setup"}}}, p.err
}

func (p *apiBootstrap) Publish(ctx context.Context, token string, repo domain.Repository, user domain.User, _ domain.BootstrapProposal, guard func(context.Context) error) (domain.BootstrapResult, error) {
	p.called++
	if err := guard(ctx); err != nil {
		return domain.BootstrapResult{}, err
	}
	return domain.BootstrapResult{Repository: repo.Owner + "/" + repo.Name, Branch: "hub-bootstrap/test", HeadSHA: strings.Repeat("b", 40), PullRequestNumber: 3, PullRequestURL: "https://github.com/octo/demo/pull/3"}, nil
}

func TestBootstrapEndpointHonorsContractAndWriteBoundary(t *testing.T) {
	for _, variant := range []string{"success", "profile-edit", "missing-draft-field", "null-draft", "draft-extra", "csrf", "origin", "session", "role", "stale", "conflict", "incomplete", "extra-field", "malformed", "method"} {
		t.Run(variant, func(t *testing.T) {
			bootstrap := &apiBootstrap{}
			handler, provider := bootstrapSetup(t, bootstrap, bootstrap)
			session, csrf := signIn(t, handler, provider)
			body := map[string]any{"csrf": csrf, "base_revision": strings.Repeat("a", 40), "digest": strings.Repeat("d", 64)}
			status := 200
			switch variant {
			case "profile-edit", "missing-draft-field", "draft-extra", "null-draft":
				draft := map[string]any{"name": "Reviewed", "description": "Display", "enabled": false, "context_sources": []string{"issue", "repository"}, "review_comments": false}
				body["profile_draft"] = draft
				if variant == "missing-draft-field" {
					delete(draft, "enabled")
					status = 400
				}
				if variant == "draft-extra" {
					draft["content"] = "arbitrary"
					status = 400
				}
				if variant == "null-draft" {
					body["profile_draft"] = nil
					status = 400
				}
			case "csrf":
				body["csrf"] = strings.Repeat("x", 43)
				status = 403
			case "origin":
				status = 403
			case "session":
				session.Value = strings.Repeat("x", 43)
				status = 401
			case "role":
				provider.access.Role = "read"
				status = 403
			case "stale":
				bootstrap.err = domain.ErrBootstrapStale
				status = 409
			case "conflict":
				bootstrap.err = domain.ErrBootstrapConflict
				status = 409
			case "incomplete":
				bootstrap.err = domain.ErrBootstrapIncomplete
				status = 409
			case "extra-field":
				body["content"] = "arbitrary source"
				status = 400
			case "malformed":
				body["digest"] = "bad"
				status = 400
			case "method":
				status = 405
			}
			encoded, _ := json.Marshal(body)
			method := "POST"
			if variant == "method" {
				method = "GET"
			}
			request := httptest.NewRequest(method, "/v1/repositories/octo/demo/bootstrap-pull-request", strings.NewReader(string(encoded)))
			request.Header.Set("Origin", origin)
			request.Header.Set("Content-Type", "application/json")
			request.AddCookie(session)
			if variant == "origin" {
				request.Header.Set("Origin", "https://other.example")
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if variant == "profile-edit" && (bootstrap.draft == nil || bootstrap.draft.Enabled || bootstrap.draft.Name != "Reviewed") {
				t.Fatal("structured choices lost before planning")
			}
			if response.Code != status {
				t.Fatalf("status %d: %s", response.Code, response.Body.String())
			}
			contractRequest := request
			if variant == "method" {
				contractRequest = httptest.NewRequest(http.MethodPost, request.URL.String(), nil)
			}
			assertContract(t, contractRequest, response)
			if variant != "success" && variant != "profile-edit" && bootstrap.called != 0 {
				t.Fatal("rejected request reached writer")
			}
			if strings.Contains(response.Body.String(), "synthetic-access") {
				t.Fatal("credential exposed")
			}
		})
	}
}
