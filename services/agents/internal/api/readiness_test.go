package api

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/r59q/hub-rearranger/services/agents/internal/domain"
)

type readinessUseCases struct {
	*domain.Service
	result domain.RepositoryReadiness
	err    error
	calls  int
}

func (s *readinessUseCases) RepositoryReadiness(ctx context.Context, _ domain.Repository) (domain.RepositoryReadiness, error) {
	s.calls++
	if _, ok := ctx.Deadline(); !ok {
		panic("missing deadline")
	}
	return s.result, s.err
}

func TestReadinessAPIHonorsContractAndSafeStates(t *testing.T) {
	for _, state := range []string{"configuration_missing", "verification_pending", "runtime_verified", "verification_failed"} {
		t.Run(state, func(t *testing.T) {
			// Arrange.
			row := domain.ProfileReadiness{ID: "codex-thorough", Revision: profileSHA, State: state, RunnerLabel: "hub-agent-codex", NextAction: "Follow the recovery guide.", Diagnostics: []domain.Diagnostic{}}
			if state != "runtime_verified" {
				row.Diagnostics = []domain.Diagnostic{{Code: "MISSING_FILE", Path: "docs/agent-workflows.md", Message: "Add required configuration."}}
			}
			if state == "runtime_verified" || state == "verification_failed" {
				model, effort, version := "gpt-6.1-sol", "high", "codex-cli 0.157.1"
				row.Evidence = &domain.RuntimeEvidence{Version: 1, Repository: "octo/demo", ProfileID: row.ID, ProfileRevision: profileSHA, RunnerLabel: row.RunnerLabel, RequestedModel: model, RequestedReasoningEffort: effort, EffectiveModel: &model, EffectiveReasoningEffort: &effort, CLIVersion: &version, RunID: 10, RunAttempt: 2, VerifiedAt: time.Now().UTC(), Outcome: "verified", ReasonCode: "verified"}
				if state == "verification_failed" {
					row.Evidence.Outcome = "failed"
					row.Evidence.ReasonCode = "request_failed"
					row.Evidence.EffectiveModel = nil
					row.Evidence.EffectiveReasoningEffort = nil
				}
			}
			service := &readinessUseCases{Service: domain.NewService(nil), result: domain.RepositoryReadiness{Repository: "octo/demo", DefaultBranch: "main", Revision: profileSHA, CatalogState: "valid", Profiles: []domain.ProfileReadiness{row}, Diagnostics: []domain.Diagnostic{}}}
			handler := NewHandler(service, slog.New(slog.NewTextHandler(io.Discard, nil)))
			for _, method := range []string{"GET", "HEAD"} {
				request := httptest.NewRequest(method, "/v1/repositories/octo/demo/readiness", nil)
				response := httptest.NewRecorder()

				// Act.
				handler.ServeHTTP(response, request)

				// Assert.
				if response.Code != 200 || response.Header().Get("Cache-Control") != "no-store" {
					t.Fatalf("response=%s", response.Body.String())
				}
				assertContractResponse(t, request, response)
				if method == "HEAD" && response.Body.Len() != 0 {
					t.Fatal("HEAD response body")
				}
				if method == "GET" && !strings.Contains(response.Body.String(), state) {
					t.Fatal("missing readiness state")
				}
			}
		})
	}
}

func TestReadinessAPIFailuresMethodsAndInvalidParameters(t *testing.T) {
	for failure, status := range map[error]int{domain.ErrAccessDenied: 403, domain.ErrRepositoryUnavailable: 404, domain.ErrRateLimited: 429, domain.ErrGitHubUnavailable: 502, context.Canceled: 502, context.DeadlineExceeded: 502, errors.New("private-sentinel"): 500} {
		t.Run(failure.Error(), func(t *testing.T) {
			// Arrange.
			service := &readinessUseCases{Service: domain.NewService(nil), err: failure}
			var logs bytes.Buffer
			handler := NewHandler(service, slog.New(slog.NewTextHandler(&logs, nil)))
			request := httptest.NewRequest("GET", "/v1/repositories/octo/demo/readiness", nil)
			response := httptest.NewRecorder()

			// Act.
			handler.ServeHTTP(response, request)

			// Assert.
			if response.Code != status || strings.Contains(logs.String()+response.Body.String(), "private-sentinel") {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
			if status == 403 && !strings.Contains(response.Body.String(), "Actions") {
				t.Fatal("missing permission guidance")
			}
			assertContractResponse(t, request, response)
		})
	}
	for _, test := range []struct {
		method, path string
		status       int
	}{{"POST", "/v1/repositories/octo/demo/readiness", 405}, {"GET", "/v1/repositories/bad_owner/demo/readiness", 400}} {
		service := &readinessUseCases{Service: domain.NewService(nil)}
		handler := NewHandler(service, slog.New(slog.NewTextHandler(io.Discard, nil)))
		response := httptest.NewRecorder()

		handler.ServeHTTP(response, httptest.NewRequest(test.method, test.path, nil))

		if response.Code != test.status || service.calls != 0 {
			t.Fatalf("status=%d calls=%d", response.Code, service.calls)
		}
	}
}
