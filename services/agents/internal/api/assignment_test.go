package api

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/r59q/hub-rearranger/services/agents/internal/domain"
)

type assignmentServiceStub struct {
	*domain.Service
	value domain.IssueAssignment
	err   error
}

func (s assignmentServiceStub) IssueAssignment(context.Context, domain.Repository, int64) (domain.IssueAssignment, error) {
	return s.value, s.err
}

func TestIssueAssignmentEndpointHonorsContractAndSafeFailures(t *testing.T) {
	sha := strings.Repeat("a", 40)
	value := domain.IssueAssignment{Repository: "octo/demo", RepositoryID: 42, Number: 3, Title: "Task", URL: "https://github.com/octo/demo/issues/3", IssueState: "open", ProfileRevision: sha, Command: "/agent assign codex-thorough@" + sha + " authority=branch-draft-pr", Requests: []domain.AssignmentRequest{{CommentID: 99, RequesterID: 7, Requester: "octocat", URL: "https://github.com/octo/demo/issues/3#issuecomment-99", ProfileRevision: sha, State: "proposal", CreatedAt: time.Now().UTC(), Run: &domain.AssignmentRun{ID: 321, Attempt: 1, URL: "https://github.com/octo/demo/actions/runs/321", Status: "completed", Conclusion: "success"}, Proposal: &domain.AssignmentProposal{Number: 5, URL: "https://github.com/octo/demo/pull/5", Branch: "agent/codex-thorough/42-99", BranchURL: "https://github.com/octo/demo/tree/agent/codex-thorough/42-99", HeadSHA: sha, State: "open", Draft: true, Check: &domain.AssignmentCheck{URL: "https://github.com/octo/demo/runs/77", Status: "completed", Conclusion: "neutral", Summary: "Repository validation: unavailable."}}}}}
	for _, variant := range []struct {
		err    error
		status int
	}{{nil, 200}, {domain.ErrAccessDenied, 403}, {domain.ErrRepositoryUnavailable, 404}, {domain.ErrRateLimited, 429}, {domain.ErrGitHubUnavailable, 502}} {
		t.Run(http.StatusText(variant.status), func(t *testing.T) {
			handler := NewHandler(assignmentServiceStub{Service: domain.NewService(nil), value: value, err: variant.err}, slog.New(slog.NewTextHandler(io.Discard, nil)))
			request := httptest.NewRequest("GET", "/v1/repositories/octo/demo/issues/3/assignment", nil)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != variant.status {
				t.Fatal(response.Code, response.Body.String())
			}
			assertContractResponse(t, request, response)
		})
	}
	for _, path := range []string{"/v1/repositories/octo/demo/issues/0/assignment", "/v1/repositories/octo/demo/issues/2147483648/assignment", "/v1/repositories/octo/demo/issues/no/assignment"} {
		handler := NewHandler(domain.NewService(nil), slog.New(slog.NewTextHandler(io.Discard, nil)))
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest("GET", path, nil))
		if response.Code != 400 {
			t.Fatal(response.Code)
		}
	}
}
