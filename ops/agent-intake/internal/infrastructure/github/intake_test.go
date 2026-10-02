package github

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/r59q/hub-rearranger/ops/agent-intake/internal/domain"
)

func TestControlledGitHubBoundaryAcceptsAndResumesWithoutDuplicateReceipt(t *testing.T) {
	service, _, fixture, event := setup(t)

	first, err := service.Intake(context.Background(), event)

	if err != nil || first.Invocation.Requester.ID != 7 || fixture.creates != 1 {
		t.Fatalf("intake failed: %v", err)
	}

	fixture.head = strings.Repeat("3", 40)
	fixture.attempts[100] = 2
	event.Attempt = 2

	resume, err := service.Intake(context.Background(), event)

	if err != nil || resume.Disposition != "resume" || resume.Invocation.BaseSHA != first.Invocation.BaseSHA ||
		resume.Invocation.AssignmentID != first.Invocation.AssignmentID || resume.Invocation.PolicyRevision != fixture.head || fixture.creates != 1 || fixture.edits != 1 {
		t.Fatalf("resume failed: %v", err)
	}
	encoded, _ := json.Marshal(resume.Invocation)
	if strings.Contains(string(encoded), "synthetic-workflow-token") || strings.Contains(string(encoded), "Synthetic fixture") {
		t.Fatal("token or display prose entered dispatch")
	}
}

func TestConcurrentDuplicateRunIDsElectOnlyOneDispatch(t *testing.T) {
	service, _, fixture, original := setup(t)
	fixture.runs = nil
	for id := int64(108); id >= 100; id-- {
		fixture.runs = append(fixture.runs, id)
		fixture.attempts[id] = 1
	}

	var wait sync.WaitGroup
	decisions := make(chan domain.Decision, 9)
	failures := make(chan error, 9)

	for id := int64(100); id <= 108; id++ {
		wait.Add(1)
		go func(id int64) {
			defer wait.Done()
			event := original
			event.RunID = id

			decision, err := service.Intake(context.Background(), event)
			if err != nil {
				failures <- err
			} else {
				decisions <- decision
			}
		}(id)
	}

	wait.Wait()
	close(decisions)
	close(failures)

	for err := range failures {
		t.Fatalf("concurrent intake: %v", err)
	}

	dispatches := 0
	for decision := range decisions {
		if decision.CanonicalRunID != 100 {
			t.Fatal("queue order changed assignment owner")
		}
		if decision.Invocation != nil {
			dispatches++
		}
	}

	if dispatches != 1 || fixture.creates != 1 {
		t.Fatal("concurrent duplicate work or receipts")
	}
}

func TestRealBoundariesRejectChangedOrUnverifiableAssignments(t *testing.T) {
	for _, variant := range []string{"deleted", "edited", "pr", "non-ancestor", "forged-run", "truncated-history", "missing-current", "role", "policy", "disabled"} {
		t.Run(variant, func(t *testing.T) {
			service, _, fixture, event := setup(t)
			fixture.variant = variant
			switch variant {
			case "role":
				fixture.role = "write"
			case "policy":
				fixture.currentCatalog = strings.ReplaceAll(fixture.currentCatalog, "reasoning_effort: high", "reasoning_effort: medium")
			case "disabled":
				fixture.currentCatalog = strings.ReplaceAll(fixture.currentCatalog, "enabled: true", "enabled: false")
			}

			_, err := service.Intake(context.Background(), event)

			if err == nil || fixture.creates != 0 {
				t.Fatal("unsafe assignment accepted")
			}
			if strings.Contains(err.Error(), "private-sentinel") {
				t.Fatal("raw GitHub error escaped")
			}
		})
	}
}

func TestLostReceiptResponseIsReconciledWithoutAnotherPost(t *testing.T) {
	service, _, fixture, event := setup(t)
	fixture.variant = "lost-post-response"

	decision, err := service.Intake(context.Background(), event)

	if err != nil || decision.Invocation.ReceiptCommentID != 900 || fixture.creates != 1 {
		t.Fatalf("ambiguous publication not reconciled: %v", err)
	}
}

func TestFailedReceiptUpdateCannotUsePreviousAttemptAsConfirmation(t *testing.T) {
	service, _, fixture, event := setup(t)

	if _, err := service.Intake(context.Background(), event); err != nil {
		t.Fatal(err)
	}

	fixture.variant = "failed-edit"
	fixture.attempts[100] = 2
	event.Attempt = 2

	decision, err := service.Intake(context.Background(), event)

	if !errors.Is(err, domain.Unavailable) || decision.Invocation != nil || fixture.creates != 1 || fixture.edits != 0 {
		t.Fatal("previous receipt confirmed a failed current-attempt publication")
	}
}

func TestExistingReceiptCannotRestoreRevokedPermissionOrProfilePolicy(t *testing.T) {
	for _, attempt := range []string{"duplicate", "rerun"} {
		for _, revocation := range []string{"role", "disabled", "removed", "changed"} {
			t.Run(attempt+"/"+revocation, func(t *testing.T) {
				service, _, fixture, event := setup(t)

				if _, err := service.Intake(context.Background(), event); err != nil {
					t.Fatal(err)
				}

				if attempt == "duplicate" {
					event.RunID = 101
					fixture.runs = append(fixture.runs, 101)
					fixture.attempts[101] = 1
				} else {
					event.Attempt = 2
					fixture.attempts[100] = 2
				}
				switch revocation {
				case "role":
					fixture.role = "write"
				case "disabled":
					fixture.currentCatalog = strings.ReplaceAll(fixture.currentCatalog, "enabled: true", "enabled: false")
				case "removed":
					fixture.currentCatalog = "schema_version: 1\nprofiles: {}\n"
				case "changed":
					fixture.currentCatalog = strings.ReplaceAll(fixture.currentCatalog, "reasoning_effort: high", "reasoning_effort: medium")
				}

				decision, err := service.Intake(context.Background(), event)

				if err == nil || decision.Invocation != nil || fixture.creates != 1 || fixture.edits != 0 {
					t.Fatal("previous receipt restored revoked authority")
				}
			})
		}
	}
}

func TestRunElectionReadsAllPages(t *testing.T) {
	service, _, fixture, event := setup(t)
	fixture.variant = "paginated-history"
	fixture.runs = []int64{100, 101}
	fixture.attempts[101] = 1
	event.RunID = 101

	decision, err := service.Intake(context.Background(), event)

	if err != nil || decision.CanonicalRunID != 100 || decision.Invocation != nil {
		t.Fatalf("older run lost across pagination: %v", err)
	}
}

func TestForgedUserReceiptCannotSupplyAuthorityOrBase(t *testing.T) {
	service, _, fixture, event := setup(t)
	fixture.receipts = []map[string]any{{"id": 999, "body": receiptPrefix + `{"assignment_id":"42:99","run_id":1}` + "\n-->", "user": map[string]any{"id": 7, "type": "User"}}}

	decision, err := service.Intake(context.Background(), event)

	if err != nil || decision.Invocation.ReceiptCommentID != 900 || fixture.creates != 1 {
		t.Fatalf("user forged receipt influenced intake: %v", err)
	}
}

func TestMalformedBotReceiptFailsClosed(t *testing.T) {
	service, _, fixture, event := setup(t)
	fixture.receipts = []map[string]any{{"id": 999, "body": receiptPrefix + `{"assignment_id":"42:99","private-sentinel":"value"}` + "\n-->", "user": map[string]any{"id": 41898282, "type": "Bot"}}}

	_, err := service.Intake(context.Background(), event)

	if !errors.Is(err, domain.HistoryUnavailable) || fixture.creates != 0 {
		t.Fatal("malformed bot state authorized work")
	}
}
