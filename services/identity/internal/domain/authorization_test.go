package domain

import (
	"context"
	"errors"
	"testing"
)

func TestAuthorizedOperationUsesCurrentVerifiedIdentity(t *testing.T) {
	service, _, provider, id, session := setup()
	called := false

	result, err := service.WithAuthorization(context.Background(), id, session.CSRF, Repository{"octo", "demo"}, AssignComment, func(_ context.Context, user User, token string) error {
		called = true
		if user != provider.user || token != session.Credentials.Access {
			t.Fatal("write did not use verified user identity")
		}
		return nil
	})

	if err != nil || !called || result.User.ID != 7 || result.RepositoryID != 42 {
		t.Fatal("authorized operation did not run")
	}

	provider.access.Role = "write"
	called = false

	_, err = service.WithAuthorization(context.Background(), id, session.CSRF, Repository{"octo", "demo"}, AssignComment, func(context.Context, User, string) error { called = true; return nil })

	if !errors.Is(err, ErrForbidden) || called {
		t.Fatal("revoked maintainer role still authorized a write")
	}
}

func TestAuthorizationFailsClosedBeforeOperation(t *testing.T) {
	for _, variant := range []string{"csrf", "session", "revoked", "repository", "role", "issues", "contents", "pull-requests", "workflows", "unknown-action", "invalid-repo", "cancelled"} {
		t.Run(variant, func(t *testing.T) {
			service, _, provider, id, session := setup()
			csrf := session.CSRF
			repo := Repository{"octo", "demo"}
			action := AssignComment
			ctx := context.Background()
			switch variant {
			case "csrf":
				csrf = randomSecret()
			case "session":
				id = randomSecret()
			case "revoked":
				provider.userError = ErrReconnect
			case "repository":
				provider.access.FullName = "other/repo"
			case "role":
				provider.access.Role = "read"
			case "issues":
				provider.access.IssuesWrite = false
			case "contents":
				action = BootstrapPullRequest
				provider.access.ContentsWrite = false
			case "pull-requests":
				action = BootstrapPullRequest
				provider.access.PullRequestsWrite = false
			case "workflows":
				action = BootstrapPullRequest
				provider.access.WorkflowsWrite = false
			case "unknown-action":
				action = "merge"
			case "invalid-repo":
				repo.Name = ".."
			case "cancelled":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			called := false

			_, err := service.WithAuthorization(ctx, id, csrf, repo, action, func(context.Context, User, string) error { called = true; return nil })

			if err == nil || called {
				t.Fatal("unauthorized operation ran")
			}
			if variant == "csrf" && provider.checks != 0 {
				t.Fatal("forged request reached GitHub")
			}
		})
	}
}

func TestBootstrapAcceptsWriteRoleWithAllRequiredPermissions(t *testing.T) {
	service, _, provider, id, session := setup()
	provider.access.Role = "write"

	_, err := service.WithAuthorization(context.Background(), id, session.CSRF, Repository{"octo", "demo"}, BootstrapPullRequest, nil)

	if err != nil {
		t.Fatal(err)
	}
}
