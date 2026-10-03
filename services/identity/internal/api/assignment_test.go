package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/r59q/hub-rearranger/services/identity/internal/api/contract"
	"github.com/r59q/hub-rearranger/services/identity/internal/domain"
	"github.com/r59q/hub-rearranger/services/identity/internal/infrastructure/sqlite"
)

type assignmentPlanner struct{ revision string }

func (p assignmentPlanner) Assignment(context.Context, domain.Repository, int64) (domain.AssignmentPlan, error) {
	return domain.AssignmentPlan{Repository: "octo/demo", RepositoryID: 42, Number: 3, Title: "Task", Revision: p.revision, Command: "/agent assign codex-thorough@" + p.revision + " authority=branch-draft-pr", Assignable: true}, nil
}

type assignmentWriter struct{ calls int }

func (w *assignmentWriter) Assign(ctx context.Context, _ string, _ domain.Repository, _ domain.User, _ domain.AssignmentPlan, guard func(context.Context) error) (domain.AssignmentResult, error) {
	if err := guard(ctx); err != nil {
		return domain.AssignmentResult{}, err
	}
	w.calls++
	return domain.AssignmentResult{Repository: "octo/demo", Number: 3, CommentID: 99, CommentURL: "https://github.com/octo/demo/issues/3#issuecomment-99"}, nil
}

func TestAssignmentAPIProtectsUserReviewAndOneUseWrite(t *testing.T) {
	for _, variant := range []string{"success", "revoked", "role", "origin", "csrf", "extra", "duplicate-cookie", "no-cookie"} {
		t.Run(variant, func(t *testing.T) {
			store, err := sqlite.Open(filepath.Join(t.TempDir(), "identity.db"), bytes.Repeat([]byte{9}, 32))
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()
			provider := &testProvider{access: domain.Access{RepositoryID: 42, FullName: "octo/demo", Role: "maintain", IssuesWrite: true}}
			writer := &assignmentWriter{}
			sha := strings.Repeat("a", 40)
			handler := NewHandler(domain.NewService(provider, store, time.Now).WithAssignments(assignmentPlanner{sha}, writer), origin, true)
			session, csrf := signIn(t, handler, provider)
			reviewPath := "/v1/repositories/octo/demo/issues/3/assignment-review"
			request, response := call(handler, "POST", reviewPath, fmt.Sprintf(`{"csrf":%q,"profile_revision":%q}`, csrf, sha), session)
			if response.Code != 200 || writer.calls != 0 {
				t.Fatal(response.Code, response.Body.String())
			}
			assertContract(t, request, response)
			var review contract.AssignmentReview
			_ = json.Unmarshal(response.Body.Bytes(), &review)
			body := fmt.Sprintf(`{"csrf":%q,"review_token":%q}`, csrf, review.ReviewToken)
			cookies := []*http.Cookie{session}
			switch variant {
			case "revoked":
				provider.failure = domain.ErrReconnect
			case "role":
				provider.access.Role = "write"
			case "csrf":
				body = fmt.Sprintf(`{"csrf":"forged","review_token":%q}`, review.ReviewToken)
			case "extra":
				body = strings.TrimSuffix(body, "}") + `,"body":"private-sentinel"}`
			case "duplicate-cookie":
				cookies = append(cookies, session)
			case "no-cookie":
				cookies = nil
			}
			path := "/v1/repositories/octo/demo/issues/3/assignment"
			request = httptest.NewRequest("POST", path, strings.NewReader(body))
			request.Header.Set("Origin", origin)
			request.Header.Set("Content-Type", "application/json")
			for _, cookie := range cookies {
				request.AddCookie(cookie)
			}
			if variant == "origin" {
				request.Header.Set("Origin", "https://foreign.example")
			}
			response = httptest.NewRecorder()
			handler.ServeHTTP(response, request)

			expected := map[string]int{"success": 200, "revoked": 401, "role": 403, "origin": 403, "csrf": 403, "extra": 400, "duplicate-cookie": 401, "no-cookie": 401}[variant]
			if response.Code != expected {
				t.Fatal(response.Code, response.Body.String())
			}
			assertContract(t, request, response)
			if variant == "success" {
				request, response = call(handler, "POST", path, body, session)
				if response.Code != 409 {
					t.Fatal("replay succeeded")
				}
				assertContract(t, request, response)
			}
			wanted := 0
			if variant == "success" {
				wanted = 1
			}
			if writer.calls != wanted {
				t.Fatal("wrong write count", writer.calls)
			}
		})
	}
}
