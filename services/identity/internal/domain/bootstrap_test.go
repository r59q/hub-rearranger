package domain

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type testBootstrapPlanner struct {
	proposal BootstrapProposal
	called   int
}

func (p *testBootstrapPlanner) Proposal(context.Context, Repository, BootstrapReview) (BootstrapProposal, error) {
	p.called++
	return p.proposal, nil
}

type testBootstrapWriter struct {
	called    bool
	operation func(context.Context, string, User, func(context.Context) error) error
}

func (w *testBootstrapWriter) Publish(ctx context.Context, token string, _ Repository, user User, _ BootstrapProposal, guard func(context.Context) error) (BootstrapResult, error) {
	w.called = true
	return BootstrapResult{}, w.operation(ctx, token, user, guard)
}

func TestBootstrapUsesAuthorizedUserAndRejectsForgedOrRevokedReviews(t *testing.T) {
	for _, variant := range []string{"success", "csrf", "revoked", "role", "stale", "phase-role", "phase-user", "phase-token"} {
		t.Run(variant, func(t *testing.T) {
			service, _, provider, id, session := setup()
			review := BootstrapReview{BaseRevision: strings.Repeat("a", 40), Digest: strings.Repeat("d", 64)}
			planner := &testBootstrapPlanner{proposal: BootstrapProposal{Repository: "octo/demo", BaseRevision: review.BaseRevision, Digest: review.Digest, Changes: []BootstrapChange{{Path: "AGENTS.md", Content: "setup"}}}}
			writer := &testBootstrapWriter{operation: func(ctx context.Context, token string, user User, guard func(context.Context) error) error {
				if token != session.Credentials.Access || user != session.User {
					t.Fatal("wrong identity passed to write")
				}
				switch variant {
				case "phase-role":
					provider.access.Role = "read"
				case "phase-user":
					provider.user.ID++
				case "phase-token":
					provider.userError = ErrReconnect
				}
				return guard(ctx)
			}}
			csrf := session.CSRF
			switch variant {
			case "csrf":
				csrf = "forged"
			case "revoked":
				provider.userError = ErrReconnect
			case "role":
				provider.access.Role = "read"
			case "stale":
				planner.proposal.Digest = strings.Repeat("e", 64)
			}
			_, err := service.WithBootstrap(planner, writer).CreateBootstrap(context.Background(), id, csrf, Repository{"octo", "demo"}, review)
			if variant == "success" {
				if err != nil || !writer.called {
					t.Fatal(err)
				}
				return
			}
			if err == nil {
				t.Fatal("unauthorized write succeeded")
			}
			if variant == "csrf" && (planner.called != 0 || writer.called || provider.checks != 0) {
				t.Fatal("forgery reached integration")
			}
			if variant == "stale" && (!errors.Is(err, ErrBootstrapStale) || writer.called) {
				t.Fatal("stale review reached writer")
			}
		})
	}
}
