package domain

import (
	"context"
	"encoding/json"
)

type snapshot struct {
	head    string
	pinned  []byte
	profile json.RawMessage
}

func (s *Service) policySnapshot(ctx context.Context, repo Repository, command Command) (snapshot, error) {
	head, err := s.repository.Head(ctx, repo)
	if err != nil {
		return snapshot{}, err
	}
	if !ValidSHA(head) {
		return snapshot{}, RevisionInvalid
	}

	ancestor, err := s.repository.Ancestor(ctx, repo, command.Revision, head)
	if err != nil {
		return snapshot{}, err
	}
	if !ancestor {
		return snapshot{}, RevisionInvalid
	}

	pinned, err := s.repository.Catalog(ctx, repo, command.Revision)
	if err != nil {
		return snapshot{}, err
	}
	current, err := s.repository.Catalog(ctx, repo, head)
	if err != nil {
		return snapshot{}, err
	}
	profile, err := s.policy.Verify(ctx, pinned, current, command.ProfileID)
	if err != nil {
		return snapshot{}, err
	}

	return snapshot{head: head, pinned: pinned, profile: profile}, nil
}

func (s *Service) recheck(ctx context.Context, repo Repository, event Event, source Source, original snapshot, invocation *Invocation) error {
	// Recheck the source and roles immediately before publishing the acceptance.
	fresh, err := s.repository.Source(ctx, repo, event)
	if err != nil {
		return err
	}
	if fresh != source {
		return SourceChanged
	}

	role, err := s.authorize(ctx, repo, event, fresh)
	if err != nil {
		return err
	}
	invocation.RequesterRole = role

	latest, err := s.repository.Head(ctx, repo)
	if err != nil {
		return err
	}
	if !ValidSHA(latest) {
		return RevisionInvalid
	}
	if latest != original.head {
		for _, revision := range []string{invocation.ProfileRevision, invocation.BaseSHA} {
			valid, err := s.repository.Ancestor(ctx, repo, revision, latest)
			if err != nil {
				return err
			}
			if !valid {
				return RevisionInvalid
			}
		}

		catalog, err := s.repository.Catalog(ctx, repo, latest)
		if err != nil {
			return err
		}
		invocation.Profile, err = s.policy.Verify(ctx, original.pinned, catalog, invocation.ProfileID)
		if err != nil {
			return err
		}
		invocation.PolicyRevision = latest
	}

	if ctx.Err() != nil {
		return Unavailable
	}
	return nil
}
