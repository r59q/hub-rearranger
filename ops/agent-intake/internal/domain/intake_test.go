package domain

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

type fakeRepository struct {
	repo                                 Repository
	source                               Source
	run                                  Run
	receipt                              *Receipt
	head                                 string
	ancestor                             bool
	roles                                map[string]string
	accepted                             int
	roleChecks                           int
	changeSource, revokeRole, changeHead bool
}

func (f *fakeRepository) Repository(context.Context, Event) (Repository, error) { return f.repo, nil }

func (f *fakeRepository) Source(context.Context, Repository, Event) (Source, error) {
	source := f.source
	if f.changeSource && f.roleChecks > 0 {
		source.Open = false
	}
	return source, nil
}

func (f *fakeRepository) Role(_ context.Context, _ Repository, login string) (User, string, error) {
	f.roleChecks++
	role := f.roles[login]
	if f.revokeRole && f.roleChecks > 1 {
		role = "read"
	}
	return User{ID: 7, Login: login}, role, nil
}

func (f *fakeRepository) Head(context.Context, Repository) (string, error) {
	if f.changeHead && f.roleChecks > 1 {
		return strings.Repeat("3", 40), nil
	}
	return f.head, nil
}

func (f *fakeRepository) Ancestor(context.Context, Repository, string, string) (bool, error) {
	return f.ancestor, nil
}

func (f *fakeRepository) Catalog(_ context.Context, _ Repository, sha string) ([]byte, error) {
	return []byte(sha), nil
}

func (f *fakeRepository) CanonicalRun(context.Context, Repository, Event, Source) (Run, error) {
	return f.run, nil
}

func (f *fakeRepository) Receipt(context.Context, Repository, Source, string) (*Receipt, error) {
	return f.receipt, nil
}

func (f *fakeRepository) Accept(_ context.Context, _ Repository, _ Source, receipt Receipt, _ Invocation) (int64, error) {
	f.accepted++
	receipt.CommentID = 900
	f.receipt = &receipt
	return receipt.CommentID, nil
}

type fakePolicy struct {
	failure        error
	calls          int
	changeOnLatest bool
}

func (f *fakePolicy) Verify(_ context.Context, _, current []byte, _ string) (json.RawMessage, error) {
	f.calls++
	if f.changeOnLatest && string(current) == strings.Repeat("3", 40) {
		return nil, ProfileChanged
	}
	return json.RawMessage(`{"adapter":{"id":"codex-chatgpt-private-runner"}}`), f.failure
}

func setup() (*Service, *fakeRepository, *fakePolicy, Event) {
	sha := strings.Repeat("1", 40)
	repo := Repository{ID: 42, Owner: "octo", Name: "demo", DefaultBranch: "main"}
	event := Event{Repository: repo, IssueNumber: 3, CommentID: 99,
		Body: "/agent assign codex-thorough@" + sha + " authority=branch-draft-pr", Requester: User{ID: 7, Login: "maintainer"},
		Actor: "maintainer", RerunActor: "maintainer", RunID: 100, Attempt: 1, WorkflowSHA: sha}
	created := time.Date(2026, 10, 1, 1, 0, 0, 0, time.UTC)
	f := &fakeRepository{repo: repo, head: sha, ancestor: true, roles: map[string]string{"maintainer": "maintain"},
		source: Source{IssueNumber: 3, CommentID: 99, Body: event.Body, Requester: event.Requester, Created: created, Updated: created, Open: true},
		run:    Run{ID: 100, RepositoryID: 42, HeadSHA: sha, Path: WorkflowPath, Event: "issue_comment", Title: RunTitle(42, 99)}}
	policy := &fakePolicy{}
	return New(f, policy), f, policy, event
}

func TestAcceptedInvocationContainsVerifiedIdentityAndStableAssignment(t *testing.T) {
	service, repository, _, event := setup()

	decision, err := service.Intake(context.Background(), event)

	if err != nil || decision.Disposition != "accepted" || repository.accepted != 1 {
		t.Fatalf("intake: %v", err)
	}
	invocation := decision.Invocation
	if invocation.AssignmentID != "42:99" || invocation.Requester != event.Requester || invocation.RequesterRole != "maintain" ||
		invocation.Authority != "branch-draft-pr" || invocation.BaseSHA != repository.head ||
		invocation.ReceiptCommentID != 900 || invocation.RequestURL != "https://github.com/octo/demo/issues/3#issuecomment-99" ||
		invocation.RunURL != "https://github.com/octo/demo/actions/runs/100/attempts/1" {
		t.Fatal("unverified or unstable invocation")
	}
}

func TestDuplicateDeliveryDoesNotDispatchAndRerunKeepsBase(t *testing.T) {
	service, repository, _, event := setup()

	first, err := service.Intake(context.Background(), event)

	if err != nil {
		t.Fatal(err)
	}

	event.RunID = 101

	duplicate, err := service.Intake(context.Background(), event)

	if err != nil || duplicate.Disposition != "duplicate" || duplicate.Invocation != nil || repository.accepted != 1 {
		t.Fatal("duplicate created work")
	}

	event.RunID, event.Attempt = 100, 2
	repository.head = strings.Repeat("2", 40)

	resume, err := service.Intake(context.Background(), event)

	if err != nil || resume.Disposition != "resume" || resume.Invocation.AssignmentID != first.Invocation.AssignmentID ||
		resume.Invocation.BaseSHA != first.Invocation.BaseSHA || resume.Invocation.PolicyRevision != repository.head || repository.receipt.CommentID != 900 {
		t.Fatal("rerun lost original assignment/base or did not revalidate current policy")
	}
}

func TestCurrentRevocationBlocksEvenDuplicateAndRerun(t *testing.T) {
	for _, variant := range []string{"duplicate", "rerun"} {
		t.Run(variant, func(t *testing.T) {
			service, repository, _, event := setup()

			_, _ = service.Intake(context.Background(), event)
			repository.roles["maintainer"] = "read"
			if variant == "duplicate" {
				event.RunID++
			} else {
				event.Attempt++
			}

			_, err := service.Intake(context.Background(), event)

			if !errors.Is(err, Denied) || repository.accepted != 1 {
				t.Fatal("old receipt restored revoked authority")
			}
		})
	}
}

func TestUnsafeSourcesAndActorsCannotPublishAcceptance(t *testing.T) {
	for _, variant := range []string{"fork", "repository", "archived", "closed", "pr", "edited-body", "edited-time", "requester", "role", "rerun-actor", "non-ancestor", "malformed", "source-revoked", "role-revoked"} {
		t.Run(variant, func(t *testing.T) {
			service, repository, _, event := setup()
			switch variant {
			case "fork":
				repository.repo.Fork = true
			case "repository":
				repository.repo.ID++
			case "archived":
				repository.repo.Archived = true
			case "closed":
				repository.source.Open = false
			case "pr":
				repository.source.PullRequest = true
			case "edited-body":
				repository.source.Body += " extra"
			case "edited-time":
				repository.source.Updated = repository.source.Updated.Add(time.Second)
			case "requester":
				repository.source.Requester.ID++
			case "role":
				repository.roles["maintainer"] = "write"
			case "rerun-actor":
				event.RerunActor = "untrusted"
			case "non-ancestor":
				repository.ancestor = false
			case "malformed":
				event.Body += "\nextra"
			case "source-revoked":
				repository.changeSource = true
			case "role-revoked":
				repository.revokeRole = true
			}

			_, err := service.Intake(context.Background(), event)

			if err == nil || repository.accepted != 0 {
				t.Fatal("unsafe request published acceptance")
			}
		})
	}
}

func TestPolicyRecheckedIfDefaultBranchMovesDuringIntake(t *testing.T) {
	service, repository, policy, event := setup()
	repository.changeHead = true
	policy.changeOnLatest = true

	_, err := service.Intake(context.Background(), event)

	if !errors.Is(err, ProfileChanged) || repository.accepted != 0 || policy.calls != 2 {
		t.Fatal("policy change raced acceptance")
	}
}

func TestMissingOrForgedPriorStateFailsClosed(t *testing.T) {
	for _, variant := range []string{"missing", "forged", "history"} {
		t.Run(variant, func(t *testing.T) {
			service, repository, _, event := setup()
			switch variant {
			case "missing":
				event.Attempt = 2
			case "forged":
				repository.receipt = &Receipt{AssignmentID: "42:99", RunID: 100, BaseSHA: repository.head, ProfileID: "other"}
			case "history":
				repository.run.Path = ".github/workflows/unrelated.yml"
			}

			_, err := service.Intake(context.Background(), event)

			if !errors.Is(err, HistoryUnavailable) || repository.accepted != 0 {
				t.Fatal("unverified replay state dispatched")
			}
		})
	}
}

func TestSeparateCommentsRemainSeparateAssignments(t *testing.T) {
	service, repository, _, event := setup()

	first, _ := service.Intake(context.Background(), event)

	event.CommentID++
	repository.source.CommentID = event.CommentID
	repository.run.Title = RunTitle(repository.repo.ID, event.CommentID)
	repository.receipt = nil

	second, err := service.Intake(context.Background(), event)

	if err != nil || first.Invocation.AssignmentID == second.Invocation.AssignmentID {
		t.Fatal("distinct requests were collapsed")
	}
}

func TestCancelledIntakeCannotPublish(t *testing.T) {
	service, repository, _, event := setup()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := service.Intake(ctx, event)

	if !errors.Is(err, Unavailable) || repository.accepted != 0 {
		t.Fatal("cancelled intake published")
	}
}

func TestCommandRejectsExtraFieldsAbbreviationsAndShellText(t *testing.T) {
	_, _, _, event := setup()
	for _, body := range []string{event.Body + " ", event.Body + "\n", event.Body + " authority=merge", strings.ReplaceAll(event.Body, strings.Repeat("1", 40), "main"), "/agent assign $(private-sentinel)"} {
		if _, err := Parse(body); !errors.Is(err, Invalid) {
			t.Fatal("ambiguous command accepted")
		}
	}
}
