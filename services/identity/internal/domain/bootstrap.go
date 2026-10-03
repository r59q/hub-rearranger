package domain

import (
	"context"
	"errors"
	"regexp"
)

var (
	ErrBootstrapStale      = errors.New("bootstrap review is stale")
	ErrBootstrapConflict   = errors.New("bootstrap state conflicts with review")
	ErrBootstrapIncomplete = errors.New("bootstrap publication is incomplete")
)

var bootstrapBase = regexp.MustCompile(`^[0-9a-f]{40}$`)
var bootstrapDigest = regexp.MustCompile(`^[0-9a-f]{64}$`)

type ProfileDraft struct {
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	Enabled        bool     `json:"enabled"`
	ContextSources []string `json:"context_sources"`
	ReviewComments bool     `json:"review_comments"`
}

type BootstrapReview struct {
	BaseRevision, Digest string
	Draft                *ProfileDraft
}

type BootstrapChange struct{ Path, BaseSHA, Content string }

// BootstrapProposal is mapped from Agents' versioned API, never a shared model
// or browser-authored file payload. Identity owns only the authorized Git write.
type BootstrapProposal struct {
	Repository, DefaultBranch, BaseRevision, Digest string
	Changes                                         []BootstrapChange
	ProfileEdit                                     bool
}

type BootstrapResult struct {
	Repository, Branch, HeadSHA, PullRequestURL string
	PullRequestNumber                           int
}

type BootstrapPlanner interface {
	Proposal(context.Context, Repository, BootstrapReview) (BootstrapProposal, error)
}

type BootstrapWriter interface {
	Publish(context.Context, string, Repository, User, BootstrapProposal, func(context.Context) error) (BootstrapResult, error)
}

func (s *Service) WithBootstrap(planner BootstrapPlanner, writer BootstrapWriter) *Service {
	s.bootstrapPlanner, s.bootstrapWriter = planner, writer
	return s
}

func (s *Service) CreateBootstrap(ctx context.Context, id, csrf string, repo Repository, review BootstrapReview) (BootstrapResult, error) {
	if !bootstrapBase.MatchString(review.BaseRevision) || !bootstrapDigest.MatchString(review.Digest) {
		return BootstrapResult{}, ErrInvalid
	}
	if s.bootstrapPlanner == nil || s.bootstrapWriter == nil {
		return BootstrapResult{}, ErrUnavailable
	}
	var result BootstrapResult
	_, err := s.WithAuthorization(ctx, id, csrf, repo, BootstrapPullRequest, func(ctx context.Context, user User, token string) error {
		proposal, err := s.bootstrapPlanner.Proposal(ctx, repo, review)
		if err != nil {
			return err
		}
		if proposal.BaseRevision != review.BaseRevision || proposal.Digest != review.Digest || proposal.Repository != repo.Owner+"/"+repo.Name || len(proposal.Changes) == 0 {
			return ErrBootstrapStale
		}
		guard := func(ctx context.Context) error {
			current, err := s.provider.User(ctx, token)
			if err != nil {
				return err
			}
			if current != user {
				return ErrReconnect
			}
			access, err := s.provider.RepositoryAccess(ctx, token, user, repo)
			if err != nil {
				return err
			}
			return checkAccess(access, repo, BootstrapPullRequest)
		}
		result, err = s.bootstrapWriter.Publish(ctx, token, repo, user, proposal, guard)
		return err
	})
	return result, err
}
