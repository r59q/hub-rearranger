package domain

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
)

// Reauthorize independently reconstructs the intake claim without publishing a
// receipt. Invocation artifacts are context, never a transferable write grant.
func (s *Service) Reauthorize(ctx context.Context, event Event, input Invocation) (Invocation, error) {
	repo, source, command, err := s.source(ctx, event)
	if err != nil {
		return Invocation{}, err
	}
	// Explicit operator exception (2026-10-02): shared addons and this public
	// repository are approved. All live maintainer/source/policy guards still apply.
	if !repo.Private && (repo.Owner != "r59q" || repo.Name != "hub-rearranger") {
		return Invocation{}, Denied
	}
	policy, err := s.policySnapshot(ctx, repo, command)
	if err != nil {
		return Invocation{}, err
	}
	canonical, err := s.repository.CanonicalRun(ctx, repo, event, source)
	if err != nil {
		return Invocation{}, err
	}
	if canonical.ID != event.RunID || canonical.RepositoryID != repo.ID || canonical.Attempt != event.Attempt || canonical.HeadSHA != event.WorkflowSHA || canonical.Path != WorkflowPath || canonical.Event != "issue_comment" || canonical.Title != RunTitle(repo.ID, source.CommentID) {
		return Invocation{}, HistoryUnavailable
	}
	claim, _, err := s.claim(ctx, repo, event, source, command, canonical, policy.head)
	if err != nil {
		return Invocation{}, err
	}
	if claim.CommentID <= 0 || input.ReceiptCommentID != claim.CommentID || input.ContractVersion != 1 || input.Operation != "assignment" || input.AssignmentID != claim.AssignmentID || input.Repository.ID != repo.ID || input.Repository.Owner != repo.Owner || input.Repository.Name != repo.Name || input.Repository.DefaultBranch != repo.DefaultBranch || input.IssueNumber != source.IssueNumber || input.RequestCommentID != source.CommentID || input.Requester != source.Requester || input.Authority != "branch-draft-pr" || input.ProfileID != command.ProfileID || input.ProfileRevision != command.Revision || input.BaseSHA != claim.BaseSHA || input.RunID != event.RunID || input.RunAttempt != event.Attempt || input.PolicyRevision != policy.head {
		return Invocation{}, Invalid
	}
	if input.SourceURL != fmt.Sprintf("%s/issues/%d", RepositoryURL(repo), source.IssueNumber) || input.RequestURL != fmt.Sprintf("%s/issues/%d#issuecomment-%d", RepositoryURL(repo), source.IssueNumber, source.CommentID) || input.RunURL != fmt.Sprintf("%s/actions/runs/%d/attempts/%d", RepositoryURL(repo), event.RunID, event.Attempt) {
		return Invocation{}, Invalid
	}
	var supplied, verified any
	if json.Unmarshal(input.Profile, &supplied) != nil || json.Unmarshal(policy.profile, &verified) != nil || !reflect.DeepEqual(supplied, verified) {
		return Invocation{}, ProfileChanged
	}
	if err := s.recheck(ctx, repo, event, source, policy, &input); err != nil {
		return Invocation{}, err
	}
	return input, nil
}
