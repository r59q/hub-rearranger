package github

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	gh "github.com/google/go-github/v74/github"
	"github.com/r59q/hub-rearranger/services/identity/internal/domain"
	"github.com/r59q/hub-rearranger/services/identity/internal/infrastructure/sqlite"
)

func TestControlledWriteIsAttributedToUserAndRechecksAccess(t *testing.T) {
	for _, variant := range []string{"role-revoked", "permission-revoked", "repository-removed", "token-revoked"} {
		t.Run(variant, func(t *testing.T) {
			provider, state := fixture(t)
			store, err := sqlite.Open(filepath.Join(t.TempDir(), "identity.db"), bytes.Repeat([]byte{9}, 32))
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = store.Close() }()
			id, csrf := strings.Repeat("s", 43), strings.Repeat("c", 43)
			session := domain.Session{User: domain.User{ID: 7, Login: "octocat"}, Credentials: domain.Credentials{Access: "ghu_synthetic-user"}, CSRF: csrf, Expires: time.Now().Add(time.Hour)}
			if err := store.PutSession(context.Background(), domain.Hash(id), session); err != nil {
				t.Fatal(err)
			}
			service := domain.NewService(provider, store, nil)
			operation := func(ctx context.Context, user domain.User, token string) error {
				comment, _, err := provider.api.WithAuthToken(token).Issues.CreateComment(ctx, "octo", "demo", 1, &gh.IssueComment{Body: gh.Ptr("Synthetic offline assignment request")})
				if err != nil {
					return err
				}
				if comment.GetUser().GetID() != user.ID {
					t.Fatal("comment attribution differs from the verified requester")
				}
				return nil
			}

			_, err = service.WithAuthorization(context.Background(), id, csrf, domain.Repository{Owner: "octo", Name: "demo"}, domain.AssignComment, operation)

			if err != nil || state.writes != 1 {
				t.Fatal("authorized user write failed")
			}
			switch variant {
			case "role-revoked":
				state.role = "write"
			case "permission-revoked":
				state.permission = "read"
			case "repository-removed":
				state.repositoryID = 100
			case "token-revoked":
				state.revoked = true
			}

			_, err = service.WithAuthorization(context.Background(), id, csrf, domain.Repository{Owner: "octo", Name: "demo"}, domain.AssignComment, operation)

			if (err == nil || (!errors.Is(err, domain.ErrForbidden) && !errors.Is(err, domain.ErrReconnect))) || state.writes != 1 {
				t.Fatal("stale authorization permitted another comment")
			}
		})
	}
}
