package domain

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type publicationFake struct {
	repo                                   *fakeRepository
	failure, evidenceFailure, patchFailure error
	revokeAt                               string
	branch, draft, comment, check          bool
	creates                                map[string]int
}

func (f *publicationFake) revoke(stage string) {
	if f.revokeAt == stage {
		f.repo.roles["maintainer"] = "read"
	}
}

func (f *publicationFake) Proposal(context.Context, Invocation) (Proposal, error) {
	return Proposal{}, f.failure
}

func (f *publicationFake) ExecutionSource(context.Context, Invocation) ([]byte, error) {
	return []byte("synthetic"), nil
}

func (f *publicationFake) VerifyProposal(context.Context, Invocation, Proposal) error {
	return f.evidenceFailure
}

func (f *publicationFake) Apply(context.Context, []byte, []byte) ([]FileChange, error) {
	f.revoke("patch")
	return []FileChange{{Path: "app.txt", Mode: "100644", Content: []byte("after")}}, f.patchFailure
}

func (f *publicationFake) CommitProposal(context.Context, Invocation, Proposal, []FileChange) (string, error) {
	f.creates["objects"]++
	return strings.Repeat("4", 40), nil
}

func (f *publicationFake) EnsureBranch(context.Context, Invocation, string) error {
	if !f.branch {
		f.branch = true
		f.creates["branch"]++
	}
	f.revoke("branch")
	return nil
}

func (f *publicationFake) EnsureDraft(_ context.Context, input Invocation, _ Proposal, head string) (Publication, error) {
	if !f.draft {
		f.draft = true
		f.creates["draft"]++
	}
	f.revoke("draft")
	return Publication{PublicationBranch(input), head, 12, "https://github.com/octo/demo/pull/12"}, nil
}

func (f *publicationFake) EnsurePublicationComment(context.Context, Invocation, Proposal, Publication) error {
	if !f.comment {
		f.comment = true
		f.creates["comment"]++
	}
	f.revoke("comment")
	return nil
}

func (f *publicationFake) EnsurePublicationCheck(context.Context, Invocation, Proposal, Publication) error {
	if !f.check {
		f.check = true
		f.creates["check"]++
	}
	f.revoke("check")
	return nil
}

func (f *publicationFake) ConfirmPublication(context.Context, Invocation, Proposal, Publication) error {
	return nil
}

func publisherSetup(t *testing.T) (*Publisher, *publicationFake, *fakeRepository, *fakePolicy, Event, Invocation) {
	t.Helper()
	service, repo, policy, event := setup()
	repo.repo.Private = true
	repo.run.Attempt = 1
	decision, err := service.Intake(context.Background(), event)
	if err != nil {
		t.Fatal(err)
	}
	f := &publicationFake{repo: repo, creates: map[string]int{}}
	return NewPublisher(service, repo, f, f, f), f, repo, policy, event, *decision.Invocation
}

func TestPublicationReconcilesExistingObjectsAndPreservesAssignment(t *testing.T) {
	publisher, f, _, _, event, input := publisherSetup(t)
	first, err := publisher.Publish(context.Background(), event, input)
	if err != nil {
		t.Fatal(err)
	}
	again, err := publisher.Publish(context.Background(), event, input)
	if err != nil || first != again {
		t.Fatalf("reconciliation changed publication: %v", err)
	}
	for _, stage := range []string{"branch", "draft", "comment", "check"} {
		if f.creates[stage] != 1 {
			t.Fatalf("duplicate %s", stage)
		}
	}
	if first.Branch != "agent/codex-thorough/42-99" {
		t.Fatal("branch lost assignment identity")
	}
}

func TestPublicationRejectsArtifactsProtectedChangesRevocationAndStaleHead(t *testing.T) {
	for _, variant := range []string{"artifact", "schema", "protected", "policy", "role", "source", "head", "duplicate"} {
		t.Run(variant, func(t *testing.T) {
			publisher, f, repo, policy, event, input := publisherSetup(t)
			switch variant {
			case "artifact":
				f.failure = HistoryUnavailable
			case "schema":
				f.evidenceFailure = HistoryUnavailable
			case "protected":
				f.patchFailure = ProtectedChange
			case "policy":
				policy.failure = ProfileChanged
			case "role":
				repo.roles["maintainer"] = "read"
			case "source":
				repo.source.Open = false
			case "head":
				repo.head = strings.Repeat("3", 40)
				input.PolicyRevision = repo.head
			case "duplicate":
				event.RunID++
			}
			_, err := publisher.Publish(context.Background(), event, input)
			if err == nil || f.creates["objects"] != 0 || f.creates["branch"] != 0 {
				t.Fatal("unverified publication wrote GitHub")
			}
			if variant == "head" && !errors.Is(err, StaleHead) {
				t.Fatalf("wrong stale-head result: %v", err)
			}
		})
	}
}

func TestAuthorizationIsRecheckedBetweenEveryPublicationPhase(t *testing.T) {
	for _, stage := range []string{"patch", "branch", "draft", "comment", "check"} {
		t.Run(stage, func(t *testing.T) {
			publisher, f, _, _, event, input := publisherSetup(t)
			f.revokeAt = stage
			_, err := publisher.Publish(context.Background(), event, input)
			if !errors.Is(err, Denied) {
				t.Fatalf("revoked publication claimed success: %v", err)
			}
			blocked := map[string]string{"patch": "objects", "branch": "draft", "draft": "comment", "comment": "check"}[stage]
			if blocked != "" && f.creates[blocked] != 0 {
				t.Fatal("write crossed revoked boundary")
			}
		})
	}
}

func TestRerunPreservesPublicationIntentAndRejectsInvalidState(t *testing.T) {
	for _, invalid := range []bool{false, true} {
		service, repo, _, event := setup()
		_, err := service.Intake(context.Background(), event)
		if err != nil {
			t.Fatal(err)
		}
		state := &PublicationIntent{ArtifactID: 50, Attempt: 1, HeadSHA: strings.Repeat("4", 40), CommentStarted: true}
		if invalid {
			state.ArtifactID = 0
		}
		repo.receipt.Publication = state
		event.Attempt = 2
		_, err = service.Intake(context.Background(), event)
		if invalid {
			if !errors.Is(err, HistoryUnavailable) {
				t.Fatal("invalid intent restored")
			}
		} else if err != nil || repo.receipt.Publication != state {
			t.Fatal("rerun dropped non-idempotent write intent")
		}
	}
}
