package domain

import (
	"context"
	"fmt"
)

type Service struct {
	repository RepositoryPort
	policy     PolicyPort
}

func New(repository RepositoryPort, policy PolicyPort) *Service {
	return &Service{repository: repository, policy: policy}
}

// Intake must run under the workflow's repository/comment concurrency group.
// Every attempt reauthorizes before deduplication; receipts never grant authority.
func (s *Service) Intake(ctx context.Context, event Event) (Decision, error) {
	repo, source, command, err := s.source(ctx, event)
	if err != nil {
		return Decision{}, err
	}

	policy, err := s.policySnapshot(ctx, repo, command)
	if err != nil {
		return Decision{}, err
	}

	canonical, err := s.repository.CanonicalRun(ctx, repo, event, source)
	if err != nil {
		return Decision{}, err
	}
	if canonical.RepositoryID != repo.ID || canonical.Event != "issue_comment" || canonical.Title != RunTitle(repo.ID, source.CommentID) ||
		canonical.Path != WorkflowPath || !ValidSHA(canonical.HeadSHA) || canonical.ID > event.RunID || canonical.ID <= 0 {
		return Decision{}, HistoryUnavailable
	}

	decision := Decision{CanonicalRunID: canonical.ID, Disposition: "duplicate"}
	if canonical.ID != event.RunID {
		return decision, nil
	}

	claim, disposition, err := s.claim(ctx, repo, event, source, command, canonical, policy.head)
	if err != nil {
		return Decision{}, err
	}

	invocation := Invocation{
		ContractVersion: 1, AssignmentID: claim.AssignmentID, Operation: "assignment", Repository: repo,
		IssueNumber: source.IssueNumber, SourceURL: fmt.Sprintf("%s/issues/%d", RepositoryURL(repo), source.IssueNumber),
		RequestCommentID: source.CommentID, RequestURL: fmt.Sprintf("%s/issues/%d#issuecomment-%d", RepositoryURL(repo), source.IssueNumber, source.CommentID),
		Requester: source.Requester, Authority: "branch-draft-pr", ProfileID: command.ProfileID, ProfileRevision: command.Revision, PolicyRevision: policy.head,
		Profile: policy.profile, BaseSHA: claim.BaseSHA, RunID: event.RunID, RunAttempt: event.Attempt,
		RunURL: fmt.Sprintf("%s/actions/runs/%d/attempts/%d", RepositoryURL(repo), event.RunID, event.Attempt),
	}

	if err := s.recheck(ctx, repo, event, source, policy, &invocation); err != nil {
		return Decision{}, err
	}

	invocation.ReceiptCommentID, err = s.repository.Accept(ctx, repo, source, claim, invocation)
	if err != nil {
		return Decision{}, err
	}

	decision.Disposition, decision.Invocation = disposition, &invocation
	return decision, nil
}
