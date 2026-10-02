package github

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	gh "github.com/google/go-github/v74/github"
	"github.com/r59q/hub-rearranger/ops/agent-intake/internal/domain"
	"github.com/r59q/hub-rearranger/ops/agent-intake/internal/infrastructure/patch"
	"github.com/r59q/hub-rearranger/ops/agent-intake/internal/infrastructure/policy"
)

func (f *publicationFixture) publish() (domain.Publication, error) {
	root, _ := filepath.Abs("../../../../..")
	python := os.Getenv("INTAKE_PYTHON")
	if python == "" {
		python = filepath.Join(root, "ops/private-runner/.venv/bin/python")
	}
	input, err := f.client.ExecutionInput(context.Background(), f.event)
	if err != nil {
		return domain.Publication{}, err
	}
	publisher := domain.NewPublisher(f.service, f.client, f.client, policy.Proposal{Executable: python, Script: filepath.Join(root, "ops/agent-intake/publication_policy.py")}, patch.Python{Executable: python, Script: filepath.Join(root, "ops/agent-intake/publication_patch.py")})
	return publisher.Publish(context.Background(), f.event, input)
}

func TestPublicationPipelineUsesRealSchemaGitAndRESTBoundaries(t *testing.T) {
	f := publicationSetup(t)
	value, err := f.publish()
	if err != nil || value != f.value() {
		t.Fatalf("publication pipeline: %v, posts=%v", err, f.posts)
	}
	if len(f.checks) != 1 || f.checks[0].GetConclusion() != "neutral" || !strings.Contains(f.pull.GetBody(), "validation: **failed**") || !f.pull.GetDraft() {
		t.Fatal("draft concealed failed validation")
	}
	for _, stage := range []string{"branch", "pr", "comment", "check"} {
		if f.posts[stage] != 1 {
			t.Fatalf("missing or duplicate %s", stage)
		}
	}
	again, err := f.publish()
	if err != nil || again != value {
		t.Fatalf("same attempt did not reconcile: %v", err)
	}
	for _, stage := range []string{"branch", "pr", "comment", "check"} {
		if f.posts[stage] != 1 {
			t.Fatalf("duplicate %s on retry", stage)
		}
	}
}

func TestPublicationReconcilesLostWriteResponsesWithoutRepeatingPOST(t *testing.T) {
	for _, stage := range []string{"branch", "pr", "comment", "check"} {
		t.Run(stage, func(t *testing.T) {
			f := publicationSetup(t)
			f.lose = stage
			_, err := f.publish()
			if err != nil {
				t.Fatalf("lost %s response not reconciled: %v", stage, err)
			}
			_, err = f.publish()
			if err != nil || f.posts[stage] != 1 {
				t.Fatalf("lost response repeated %s: %v", stage, err)
			}
		})
	}
}

func TestPublicationResumesAfterBranchCreationAndFailedPRCreation(t *testing.T) {
	f := publicationSetup(t)
	f.fail = "pr"
	_, err := f.publish()
	if err == nil || f.ref == nil || f.pull != nil || len(f.checks) != 0 {
		t.Fatal("partial publication claimed success")
	}
	f.fail = ""
	_, err = f.publish()
	if err != nil || f.posts["branch"] != 1 || f.posts["pr"] != 2 || f.pull == nil {
		t.Fatalf("partial publication not safely resumed: %v", err)
	}
}

func TestNonIdempotentMissingWritesRequireReconciliation(t *testing.T) {
	for _, stage := range []string{"comment", "check"} {
		t.Run(stage, func(t *testing.T) {
			f := publicationSetup(t)
			f.fail = stage
			_, err := f.publish()
			if err == nil {
				t.Fatal("failed write claimed success")
			}
			f.fail = ""
			_, err = f.publish()
			if !errors.Is(err, domain.HistoryUnavailable) || f.posts[stage] != 1 {
				t.Fatalf("ambiguous %s was repeated: %v", stage, err)
			}
		})
	}
}

func TestPublicationRejectsForgedArtifactsAndUnsuccessfulOrigin(t *testing.T) {
	for _, variant := range []string{"digest", "origin", "expired", "failed-job", "identity"} {
		t.Run(variant, func(t *testing.T) {
			f := publicationSetup(t)
			f.artifactVariant = variant
			if variant == "identity" {
				var value map[string]any
				_ = json.Unmarshal(f.proposal.Result, &value)
				value["assignment_id"] = "42:100"
				f.proposal.Result, _ = json.Marshal(value)
				f.archives[51] = executionArchive(t, map[string][]byte{"result.json": f.proposal.Result, "proposal.patch": f.proposal.Patch, "summary.json": f.proposal.Summary})
			}
			_, err := f.publish()
			if err == nil || f.posts["objects"] != 0 || f.ref != nil {
				t.Fatal("forged artifact wrote Git objects or branch")
			}
		})
	}
}

func TestRecoveryRejectsMovedHeadEditedPRAndForgedCheck(t *testing.T) {
	for _, variant := range []string{"head", "body", "closed", "fork", "check"} {
		t.Run(variant, func(t *testing.T) {
			f := publicationSetup(t)
			if _, err := f.publish(); err != nil {
				t.Fatal(err)
			}
			switch variant {
			case "head":
				f.ref.Object.SHA = gh.Ptr(strings.Repeat("7", 40))
			case "body":
				f.pull.Body = gh.Ptr("forged assignment")
			case "closed":
				f.pull.State = gh.Ptr("closed")
			case "fork":
				f.pull.Head.Repo.ID = gh.Ptr(int64(43))
			case "check":
				f.checks[0].App.ID = gh.Ptr(int64(99))
			}
			_, err := f.publish()
			if err == nil || f.posts["branch"] != 1 || f.posts["pr"] != 1 || f.posts["comment"] != 1 || f.posts["check"] != 1 {
				t.Fatal("recovery overwrote or claimed unrelated publication")
			}
		})
	}
}

func TestRerunRecoversPriorProposalAndConcurrentOldAttemptCannotWrite(t *testing.T) {
	f := publicationSetup(t)
	if _, err := f.publish(); err != nil {
		t.Fatal(err)
	}
	oldEvent, oldInput := f.event, f.input
	f.event.Attempt = 2
	f.base.attempts[100] = 2
	decision, err := f.service.Intake(context.Background(), f.event)
	if err != nil {
		t.Fatal(err)
	}
	serialized, _ := json.Marshal(decision.Invocation)
	_ = json.Unmarshal(serialized, &f.input)
	f.archives[52] = executionArchive(t, map[string][]byte{"invocation.json": serialized})
	root, _ := filepath.Abs("../../../../..")
	python := filepath.Join(root, "ops/private-runner/.venv/bin/python")
	publisher := domain.NewPublisher(f.service, f.client, f.client, policy.Proposal{Executable: python, Script: filepath.Join(root, "ops/agent-intake/publication_policy.py")}, patch.Python{Executable: python, Script: filepath.Join(root, "ops/agent-intake/publication_patch.py")})
	var oldErr, newErr error
	var wait sync.WaitGroup
	wait.Add(2)
	go func() { defer wait.Done(); _, oldErr = publisher.Publish(context.Background(), oldEvent, oldInput) }()
	go func() { defer wait.Done(); _, newErr = f.publish() }()
	wait.Wait()
	if oldErr == nil || newErr != nil {
		t.Fatalf("attempt reconciliation: old %v, new %v", oldErr, newErr)
	}
	for _, stage := range []string{"branch", "pr", "comment", "check"} {
		if f.posts[stage] != 1 {
			t.Fatalf("rerun/concurrent attempt duplicated %s", stage)
		}
	}
}

func TestPublicationRechecksLiveRevocationAndDefaultHead(t *testing.T) {
	for _, variant := range []string{"role", "profile", "head"} {
		t.Run(variant, func(t *testing.T) {
			f := publicationSetup(t)
			switch variant {
			case "role":
				f.base.role = "read"
			case "profile":
				f.base.currentCatalog = strings.Replace(fixtureCatalog(), "enabled: true", "enabled: false", 1)
			case "head":
				f.base.head = strings.Repeat("8", 40)
			}
			_, err := f.publish()
			if err == nil || f.posts["objects"] != 0 || f.ref != nil {
				t.Fatal("current revocation or stale head allowed publication")
			}
			if variant == "head" && !errors.Is(err, domain.StaleHead) {
				t.Fatalf("wrong stale result: %v", err)
			}
		})
	}
}
