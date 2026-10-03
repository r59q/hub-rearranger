package domain

import (
	"context"
	"regexp"
	"strings"
)

var ownerPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]{0,38}$`)
var repoPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,100}$`)

func ValidRepository(repo Repository) bool {
	return ownerPattern.MatchString(repo.Owner) && repoPattern.MatchString(repo.Name) && repo.Name != "." && repo.Name != ".."
}

// WithAuthorization is the mandatory boundary for future GitHub writes. The
// operation executes inside the session lock with the verified user's token,
// immediately after live repository authorization. Tokens never leave this process.
// A public preflight result is context for a UI, never a reusable write permit.
func (s *Service) WithAuthorization(ctx context.Context, id, csrf string, repo Repository, action Action, operation func(context.Context, User, string) error) (Authorization, error) {
	if !s.Enabled() {
		return Authorization{}, ErrUnavailable
	}
	if !ValidRepository(repo) || (action != AssignComment && action != BootstrapPullRequest) {
		return Authorization{}, ErrInvalid
	}
	if len(id) != 43 {
		return Authorization{}, ErrReconnect
	}

	var result Authorization
	err := s.store.WithSession(ctx, Hash(id), func(session *Session) error {
		if err := checkCSRF(session, csrf); err != nil {
			return err
		}
		if err := s.current(ctx, session); err != nil {
			return err
		}

		access, err := s.provider.RepositoryAccess(ctx, session.Credentials.Access, session.User, repo)
		if err != nil {
			return err
		}
		if err := checkAccess(access, repo, action); err != nil {
			return err
		}

		result = Authorization{User: session.User, RepositoryID: access.RepositoryID, Repository: access.FullName, Action: action, CheckedAt: s.now()}
		if ctx.Err() != nil {
			return ErrUnavailable
		}
		if operation != nil {
			return operation(ctx, session.User, session.Credentials.Access)
		}
		return nil
	})

	return result, err
}

func checkAccess(access Access, repo Repository, action Action) error {
	if access.RepositoryID <= 0 || !strings.EqualFold(access.FullName, repo.Owner+"/"+repo.Name) {
		return ErrForbidden
	}
	if action == AssignComment {
		if (access.Role != "maintain" && access.Role != "admin") || !access.IssuesWrite {
			return ErrForbidden
		}
	} else if (access.Role != "write" && access.Role != "maintain" && access.Role != "admin") || !access.ContentsWrite || !access.PullRequestsWrite || !access.WorkflowsWrite {
		return ErrForbidden
	}
	return nil
}
