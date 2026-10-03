// Package report provides fixed safe GitHub-visible outcomes.
package report

import (
	"errors"
	"fmt"

	"github.com/r59q/hub-rearranger/ops/agent-intake/internal/domain"
)

func Rejected(err error) string {
	var code domain.Failure
	if !errors.As(err, &code) {
		code = domain.Unavailable
	}

	guidance := map[domain.Failure]string{
		domain.Invalid:            "Post the documented single-line assignment with a full lowercase commit SHA.",
		domain.Denied:             "Only current repository maintainers/admins may assign or rerun. Check repository access.",
		domain.SourceChanged:      "Use a new, unedited comment on an open issue in this repository; fresh PR assignments are unsupported.",
		domain.ProfileInvalid:     "Fix the pinned/current catalog using the canonical profile validator, then post a new request.",
		domain.ProfileDisabled:    "The pinned or current profile is disabled. Review current repository policy.",
		domain.ProfileChanged:     "Execution policy changed or the adapter is unsupported. Review the current profile and post a new request.",
		domain.RevisionInvalid:    "Pin a full commit in the current default-branch history. Rewritten history requires a new request.",
		domain.Unavailable:        "GitHub could not be verified. Check permissions and rate limits, then rerun only the original authorize job and its dependent jobs; avoid Re-run all jobs to preserve proposal artifacts.",
		domain.HistoryUnavailable: "Prior run/receipt state could not be verified. Restore visibility; before a new assignment, reconcile the original side effects and confirm its private workload is gone.",
	}

	message, exists := guidance[code]
	if !exists {
		code, message = domain.Unavailable, guidance[domain.Unavailable]
	}
	return fmt.Sprintf("## Agent assignment rejected\n\n**%s**: %s\n", code, message)
}

func Accepted(decision domain.Decision, repository domain.Repository) string {
	if decision.Invocation == nil {
		return fmt.Sprintf("## Duplicate assignment delivery\n\nNo new dispatch. Rerun only the authorize job and its dependent jobs in the [original workflow](%s/actions/runs/%d) to resume this assignment. Avoid Re-run all jobs to preserve proposal artifacts.\n", domain.RepositoryURL(repository), decision.CanonicalRunID)
	}
	return fmt.Sprintf("## Agent assignment %s\n\nAssignment `%s`; profile `%s@%s`; authority `branch-draft-pr`.\n\nVerified input is ready for `codex-chatgpt-private-runner`. Execution requires independent current authorization, matching runtime evidence, operator enablement, and prior-attempt reconciliation. Intake acceptance does not prove runtime readiness.\n", decision.Disposition, decision.Invocation.AssignmentID, decision.Invocation.ProfileID, decision.Invocation.ProfileRevision)
}
