package domain

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

type assignmentPlannerStub struct{ plan AssignmentPlan }

func (p *assignmentPlannerStub) Assignment(context.Context, Repository, int64) (AssignmentPlan, error) {
	return p.plan, nil
}

type assignmentWriterStub struct {
	calls     int
	operation func(context.Context, string, User, func(context.Context) error) error
}

func (w *assignmentWriterStub) Assign(ctx context.Context, token string, _ Repository, user User, _ AssignmentPlan, guard func(context.Context) error) (AssignmentResult, error) {
	w.calls++
	return AssignmentResult{}, w.operation(ctx, token, user, guard)
}

func assignmentSetup() (*Service, *fakeProvider, string, Session, *assignmentPlannerStub, *assignmentWriterStub) {
	service, _, provider, id, session := setup()
	planner := &assignmentPlannerStub{AssignmentPlan{Repository: "octo/demo", RepositoryID: provider.access.RepositoryID, Number: 3, Revision: strings.Repeat("a", 40), Command: "/agent assign codex-thorough@" + strings.Repeat("a", 40) + " authority=branch-draft-pr", Assignable: true}}
	writer := &assignmentWriterStub{operation: func(ctx context.Context, _ string, _ User, guard func(context.Context) error) error {
		return guard(ctx)
	}}
	return service.WithAssignments(planner, writer), provider, id, session, planner, writer
}

func TestAssignmentRequiresFreshAuthorizationAndConsumesReview(t *testing.T) {
	for _, variant := range []string{"success", "csrf", "revoked", "role", "stale", "disabled", "expired", "wrong-repo", "phase-role", "phase-user", "uncertain"} {
		t.Run(variant, func(t *testing.T) {
			service, provider, id, session, planner, writer := assignmentSetup()
			review, err := service.ReviewAssignment(context.Background(), id, session.CSRF, Repository{"octo", "demo"}, 3, planner.plan.Revision)
			if err != nil || writer.calls != 0 {
				t.Fatal("review wrote GitHub", err)
			}
			csrf := session.CSRF
			repo := Repository{"octo", "demo"}
			switch variant {
			case "csrf":
				csrf = "forged"
			case "revoked":
				provider.userError = ErrReconnect
			case "role":
				provider.access.Role = "write"
			case "stale":
				planner.plan.Revision = strings.Repeat("b", 40)
			case "disabled":
				planner.plan.Assignable = false
				planner.plan.Command = "disabled"
			case "expired":
				service.now = func() time.Time { return review.Expires.Add(time.Second) }
			case "wrong-repo":
				repo.Name = "other"
			}
			writer.operation = func(ctx context.Context, token string, user User, guard func(context.Context) error) error {
				if token != session.Credentials.Access || user != session.User {
					t.Fatal("wrong write identity")
				}
				if variant == "phase-role" {
					provider.access.Role = "read"
				}
				if variant == "phase-user" {
					provider.user.ID++
				}
				if variant == "uncertain" {
					return ErrAssignmentUncertain
				}
				return guard(ctx)
			}
			_, err = service.CreateAssignment(context.Background(), id, csrf, repo, 3, review.Token)
			if (variant == "success") != (err == nil) {
				t.Fatalf("outcome %v", err)
			}
			calls := writer.calls
			_, retryErr := service.CreateAssignment(context.Background(), id, session.CSRF, repo, 3, review.Token)
			if !errors.Is(retryErr, ErrAssignmentReviewUsed) || writer.calls != calls {
				t.Fatal("review replay reached writer", retryErr)
			}
		})
	}
}

func TestConcurrentAssignmentReviewSubmissionsWriteOnce(t *testing.T) {
	service, _, id, session, planner, writer := assignmentSetup()
	review, err := service.ReviewAssignment(context.Background(), id, session.CSRF, Repository{"octo", "demo"}, 3, planner.plan.Revision)
	if err != nil {
		t.Fatal(err)
	}
	var wait sync.WaitGroup
	wait.Add(2)
	outcomes := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() {
			defer wait.Done()
			_, err := service.CreateAssignment(context.Background(), id, session.CSRF, Repository{"octo", "demo"}, 3, review.Token)
			outcomes <- err
		}()
	}
	wait.Wait()
	close(outcomes)
	successes := 0
	for err := range outcomes {
		if err == nil {
			successes++
		} else if !errors.Is(err, ErrAssignmentReviewUsed) {
			t.Fatal(err)
		}
	}
	if successes != 1 || writer.calls != 1 {
		t.Fatalf("writes=%d successes=%d", writer.calls, successes)
	}
}

func TestAssignmentReviewChecksProfileAndAccess(t *testing.T) {
	for _, variant := range []string{"disabled", "stale", "role", "csrf", "closed"} {
		t.Run(variant, func(t *testing.T) {
			service, provider, id, session, planner, writer := assignmentSetup()
			revision, csrf := planner.plan.Revision, session.CSRF
			switch variant {
			case "disabled", "closed":
				planner.plan.Assignable = false
			case "stale":
				revision = strings.Repeat("b", 40)
			case "role":
				provider.access.Role = "write"
			case "csrf":
				csrf = "forged"
			}
			_, err := service.ReviewAssignment(context.Background(), id, csrf, Repository{"octo", "demo"}, 3, revision)
			if err == nil || writer.calls != 0 {
				t.Fatal("invalid review succeeded")
			}
		})
	}
}

type failedSessionCommit struct{ Store }

func (s failedSessionCommit) WithSession(ctx context.Context, id string, operation func(*Session) error) error {
	if err := s.Store.WithSession(ctx, id, operation); err != nil {
		return err
	}
	return ErrUnavailable
}

func TestAssignmentConfirmedCommentWithSessionFailureIsUncertain(t *testing.T) {
	service, _, id, session, planner, writer := assignmentSetup()
	review, err := service.ReviewAssignment(context.Background(), id, session.CSRF, Repository{"octo", "demo"}, 3, planner.plan.Revision)
	if err != nil {
		t.Fatal(err)
	}
	writer.operation = func(context.Context, string, User, func(context.Context) error) error { return nil }
	service.assignmentWriter = confirmedAssignmentWriter{writer}
	service.store = failedSessionCommit{service.store}
	_, err = service.CreateAssignment(context.Background(), id, session.CSRF, Repository{"octo", "demo"}, 3, review.Token)
	if !errors.Is(err, ErrAssignmentUncertain) || writer.calls != 1 {
		t.Fatal(err, writer.calls)
	}
}

type confirmedAssignmentWriter struct{ *assignmentWriterStub }

func (w confirmedAssignmentWriter) Assign(ctx context.Context, token string, repo Repository, user User, plan AssignmentPlan, guard func(context.Context) error) (AssignmentResult, error) {
	_, err := w.assignmentWriterStub.Assign(ctx, token, repo, user, plan, guard)
	return AssignmentResult{CommentID: 99}, err
}
