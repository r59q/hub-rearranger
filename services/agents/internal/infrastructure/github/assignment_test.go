package github

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/r59q/hub-rearranger/services/agents/internal/domain"
)

func TestAssignmentProjectionCorroboratesGitHubProposalAndNeutralValidation(t *testing.T) {
	f := assignmentFixtureFor(t)
	result, err := f.reader.ReadAssignment(context.Background(), domain.Repository{Owner: "octo", Name: "demo"}, 3)
	if err != nil || len(result.Requests) != 1 {
		t.Fatal(result, err)
	}
	request := result.Requests[0]
	if request.State != "proposal" || request.Proposal == nil || request.Proposal.Check == nil || request.Proposal.Check.Conclusion != "neutral" || request.Proposal.Check.Summary != "Repository validation: failed. Human review is required." || request.Proposal.Branch != "agent/codex-thorough/42-99" {
		t.Fatal(request)
	}
}

func TestAssignmentProjectionRejectsForgedChangedOrUnavailableEvidence(t *testing.T) {
	for _, variant := range []string{"fork-pr", "spoof-bot", "moved-ref", "commit-message", "commit-parent", "check-origin", "check-summary", "artifact-origin", "expired-artifact", "missing-artifact", "duplicate-artifact", "corrupt-archive", "invalid-result", "result-revision", "patch-digest", "invocation-requester", "invocation-source", "runner-labels", "incomplete-runs"} {
		t.Run(variant, func(t *testing.T) {
			f := assignmentFixtureFor(t)
			f.variant = variant
			switch variant {
			case "fork-pr":
				f.pr["head"].(map[string]any)["repo"] = map[string]any{"id": 43}
			case "spoof-bot":
				f.pr["user"] = map[string]any{"id": 7, "type": "User"}
			case "moved-ref":
				f.ref["object"].(map[string]any)["sha"] = strings.Repeat("e", 40)
			case "commit-message":
				f.commit["message"] = "forged"
			case "commit-parent":
				f.commit["parents"] = []any{map[string]any{"sha": strings.Repeat("e", 40)}}
			case "check-origin":
				f.check["app"] = map[string]any{"id": 9}
			case "check-summary":
				f.check["output"] = map[string]any{"summary": "private-sentinel"}
			case "artifact-origin":
				f.artifactOrigin["head_repository_id"] = 43
			case "invalid-result":
				f.result["secret"] = "private-sentinel"
			case "result-revision":
				f.result["profile_revision"] = strings.Repeat("e", 40)
			case "patch-digest":
				f.patch = []byte("changed")
			case "invocation-requester":
				f.invocation["requester"] = map[string]any{"id": 9, "login": "foreign"}
			case "invocation-source":
				f.invocation["request_url"] = "https://github.com/octo/demo/issues/4#issuecomment-99"
			case "runner-labels":
				f.jobs[2]["labels"] = []string{"ubuntu-latest"}
			}
			result, err := f.reader.ReadAssignment(context.Background(), domain.Repository{Owner: "octo", Name: "demo"}, 3)
			if err != nil || len(result.Requests) != 1 || result.Requests[0].State != "unavailable" || result.Requests[0].Proposal != nil || strings.Contains(result.Requests[0].Summary, "private-sentinel") {
				t.Fatal(result, err)
			}
		})
	}
}

func TestAssignmentProjectionKeepsPendingBlockedAndMissingCheckExplicit(t *testing.T) {
	for _, variant := range []string{"no-run", "blocked", "no-check", "canonical-run"} {
		t.Run(variant, func(t *testing.T) {
			f := assignmentFixtureFor(t)
			f.variant = variant
			if variant == "blocked" {
				f.jobs[2]["conclusion"] = "skipped"
			}
			if variant == "canonical-run" {
				f.run["status"] = "in_progress"
				f.run["conclusion"] = ""
			}
			result, err := f.reader.ReadAssignment(context.Background(), domain.Repository{Owner: "octo", Name: "demo"}, 3)
			if err != nil {
				t.Fatal(err)
			}
			request := result.Requests[0]
			switch variant {
			case "no-run":
				if request.State != "requested" || request.Run != nil {
					t.Fatal(request)
				}
			case "blocked":
				if request.State != "blocked" || request.Proposal != nil {
					t.Fatal(request)
				}
			case "no-check":
				if request.Proposal == nil || request.Proposal.Check != nil {
					t.Fatal(request)
				}
			case "canonical-run":
				if request.Run == nil || request.Run.ID != 320 {
					t.Fatal(request)
				}
			}
		})
	}
}

func TestAssignmentProjectionFetchesFreshStateAndRejectsSourceOrHistoryErrors(t *testing.T) {
	f := assignmentFixtureFor(t)
	f.variant = "no-pr"
	f.run["status"] = "queued"
	first, err := f.reader.ReadAssignment(context.Background(), domain.Repository{Owner: "octo", Name: "demo"}, 3)
	if err != nil || first.Requests[0].State != "queued" {
		t.Fatal(first, err)
	}
	f.run["status"] = "in_progress"
	f.comment["updated_at"] = time.Date(2026, 10, 3, 12, 1, 0, 0, time.UTC)
	second, err := f.reader.ReadAssignment(context.Background(), domain.Repository{Owner: "octo", Name: "demo"}, 3)
	if err != nil || second.Requests[0].State != "running" || !second.Requests[0].Edited {
		t.Fatal(second, err)
	}
	f.variant = "incomplete-comments"
	if _, err = f.reader.ReadAssignment(context.Background(), domain.Repository{Owner: "octo", Name: "demo"}, 3); err == nil {
		t.Fatal("incomplete history accepted")
	}
	f.variant = ""
	f.issue["pull_request"] = map[string]any{"url": "pull"}
	if _, err = f.reader.ReadAssignment(context.Background(), domain.Repository{Owner: "octo", Name: "demo"}, 3); err != domain.ErrRepositoryUnavailable {
		t.Fatal("PR source accepted", err)
	}
}
