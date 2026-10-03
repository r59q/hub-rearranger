package domain

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

var (
	ErrAssignmentStale      = errors.New("assignment review is stale")
	ErrAssignmentUncertain  = errors.New("assignment comment outcome is uncertain")
	ErrAssignmentReviewUsed = errors.New("assignment review expired or consumed")
)

// AssignmentPlan is an ephemeral public Agents projection. It never carries credentials.
type AssignmentPlan struct {
	Repository                          string
	RepositoryID, Number, LastCommentID int64
	Title, Revision, Command            string
	Assignable                          bool
}

type AssignmentReview struct {
	Token   string
	Plan    AssignmentPlan
	User    User
	Expires time.Time
}

type AssignmentResult struct {
	Repository        string
	Number, CommentID int64
	CommentURL        string
}

type AssignmentPlanner interface {
	Assignment(context.Context, Repository, int64) (AssignmentPlan, error)
}

type AssignmentWriter interface {
	Assign(context.Context, string, Repository, User, AssignmentPlan, func(context.Context) error) (AssignmentResult, error)
}

func (s *Service) WithAssignments(planner AssignmentPlanner, writer AssignmentWriter) *Service {
	s.assignmentPlanner, s.assignmentWriter = planner, writer
	return s
}

func (s *Service) ReviewAssignment(ctx context.Context, id, csrf string, repo Repository, number int64, revision string) (AssignmentReview, error) {
	result := AssignmentReview{}
	if number < 1 || number > 2147483647 || !bootstrapBase.MatchString(revision) {
		return result, ErrInvalid
	}
	if s.assignmentPlanner == nil {
		return result, ErrUnavailable
	}
	_, err := s.WithAuthorization(ctx, id, csrf, repo, AssignComment, func(ctx context.Context, user User, _ string) error {
		plan, err := s.assignmentPlanner.Assignment(ctx, repo, number)
		if err != nil {
			return err
		}
		if !validAssignmentPlan(plan, repo, number) || !plan.Assignable || plan.Revision != revision {
			return ErrAssignmentStale
		}
		result = AssignmentReview{Token: randomSecret(), Plan: plan, User: user, Expires: s.now().Add(IntentLifetime)}
		return nil
	})
	if err != nil {
		return AssignmentReview{}, err
	}
	data, err := json.Marshal(result)
	if err != nil {
		return AssignmentReview{}, ErrUnavailable
	}
	// Separate purpose namespace; storage encrypts this short-lived review. Only
	// a browser-bound one-use intent is stored, never a competing assignment/run.
	err = s.store.PutIntent(ctx, "assignment:"+Hash(result.Token), Intent{Binding: Hash(id), Verifier: string(data), Expires: result.Expires})
	return result, err
}

func validAssignmentPlan(plan AssignmentPlan, repo Repository, number int64) bool {
	return plan.Repository == repo.Owner+"/"+repo.Name && plan.RepositoryID > 0 && plan.Number == number && bootstrapBase.MatchString(plan.Revision) && plan.Command == fmt.Sprintf("/agent assign codex-thorough@%s authority=branch-draft-pr", plan.Revision) && plan.LastCommentID >= 0
}

func (s *Service) CreateAssignment(ctx context.Context, id, csrf string, repo Repository, number int64, token string) (AssignmentResult, error) {
	result := AssignmentResult{}
	if !s.Enabled() || s.assignmentPlanner == nil || s.assignmentWriter == nil {
		return result, ErrUnavailable
	}
	if !ValidRepository(repo) || number < 1 || number > 2147483647 || len(token) != 43 {
		return result, ErrInvalid
	}
	if len(id) != 43 {
		return result, ErrReconnect
	}
	// Consume before the session transaction so even a lost POST response, restart,
	// failed authorization or repeated form cannot silently create another comment.
	intent, err := s.store.TakeIntent(ctx, "assignment:"+Hash(token), Hash(id))
	if errors.Is(err, ErrForgery) {
		return result, ErrAssignmentReviewUsed
	}
	if err != nil {
		return result, err
	}
	var review AssignmentReview
	if json.Unmarshal([]byte(intent.Verifier), &review) != nil || !s.now().Before(intent.Expires) || !validAssignmentPlan(review.Plan, repo, number) {
		return result, ErrAssignmentReviewUsed
	}
	_, err = s.WithAuthorization(ctx, id, csrf, repo, AssignComment, func(ctx context.Context, user User, accessToken string) error {
		if user != review.User {
			return ErrReconnect
		}
		plan, err := s.assignmentPlanner.Assignment(ctx, repo, number)
		if err != nil {
			return err
		}
		if !validAssignmentPlan(plan, repo, number) || plan.RepositoryID != review.Plan.RepositoryID || plan.Revision != review.Plan.Revision {
			return ErrAssignmentStale
		}
		guard := func(ctx context.Context) error {
			current, err := s.provider.User(ctx, accessToken)
			if err != nil {
				return err
			}
			if current != user {
				return ErrReconnect
			}
			access, err := s.provider.RepositoryAccess(ctx, accessToken, user, repo)
			if err != nil {
				return err
			}
			if access.RepositoryID != review.Plan.RepositoryID {
				return ErrForbidden
			}
			return checkAccess(access, repo, AssignComment)
		}
		result, err = s.assignmentWriter.Assign(ctx, accessToken, repo, user, review.Plan, guard)
		return err
	})
	if err != nil && result.CommentID > 0 {
		return result, ErrAssignmentUncertain
	}

	return result, err
}
