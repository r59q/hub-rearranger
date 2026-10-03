package github

import (
	"encoding/json"
	"fmt"

	"github.com/r59q/hub-rearranger/ops/agent-intake/internal/domain"
)

type assignmentProvenance struct {
	Source           string `json:"source"`
	RequestCommentID int64  `json:"request_comment_id"`
	Request          string `json:"request"`
	ProfileID        string `json:"profile_id"`
	ProfileRevision  string `json:"profile_revision"`
	RequesterID      int64  `json:"requester_id"`
	Requester        string `json:"requester"`
	Authority        string `json:"authority"`
	Run              string `json:"run"`
}

func validationOutcome(proposal domain.Proposal) string {
	var result struct {
		Validation []struct {
			Outcome string `json:"outcome"`
		} `json:"validation"`
	}
	if json.Unmarshal(proposal.Result, &result) != nil || len(result.Validation) != 1 {
		return "unavailable"
	}
	switch result.Validation[0].Outcome {
	case "passed", "failed", "unavailable":
		return result.Validation[0].Outcome
	}
	return "unavailable"
}

func publicationProvenance(input domain.Invocation, proposal domain.Proposal, head string) string {
	assignment := assignmentProvenance{input.SourceURL, input.RequestCommentID, input.RequestURL, input.ProfileID, input.ProfileRevision, input.Requester.ID, input.Requester.Login, input.Authority, proposal.Invocation.RunURL}
	publication := map[string]any{"version": 1, "assignment_id": input.AssignmentID, "base_sha": input.BaseSHA, "head_sha": head, "artifact_id": proposal.ArtifactID, "proposal_attempt": proposal.Attempt, "patch_sha256": patchDigest(proposal)}
	a, _ := json.Marshal(assignment)
	b, _ := json.Marshal(publication)
	return "<!-- agent-assignment:v1\n" + string(a) + "\n-->\n\n<!-- agent-publication:v1\n" + string(b) + "\n-->"
}

func draftBody(input domain.Invocation, proposal domain.Proposal, head string) string {
	return fmt.Sprintf("## Agent assignment\n\nPatch proposal for [issue #%d](%s); [assignment request](%s).\n\nProfile: `%s@%s`; requester: `%s` (account %d); authority: `branch-draft-pr`.\n\nBase: `%s`; proposal head: `%s`.\n\nRepository validation: **%s**. This draft requires human review; a ready proposal does not imply validation passed.\n\n[Originating workflow attempt](%s); [verified patch artifact](%s/actions/runs/%d/artifacts/%d).\n\n%s", input.IssueNumber, input.SourceURL, input.RequestURL, input.ProfileID, input.ProfileRevision, input.Requester.Login, input.Requester.ID, input.BaseSHA, head, validationOutcome(proposal), proposal.Invocation.RunURL, domain.RepositoryURL(input.Repository), input.RunID, proposal.ArtifactID, publicationProvenance(input, proposal, head))
}

func draftTitle(input domain.Invocation) string {
	return fmt.Sprintf("Agent patch proposal for issue #%d", input.IssueNumber)
}

func publicationComment(input domain.Invocation, proposal domain.Proposal, value domain.Publication) string {
	return publicationProvenance(input, proposal, value.HeadSHA) + fmt.Sprintf("\n\nDraft patch proposal: [PR #%d](%s). Repository validation: **%s**.\n\n[Assignment request](%s); [proposal workflow attempt](%s). Review the patch and validation evidence before taking action.", value.PRNumber, value.PRURL, validationOutcome(proposal), input.RequestURL, proposal.Invocation.RunURL)
}
