package domain

import "context"

func (s *Service) claim(ctx context.Context, repo Repository, event Event, source Source, command Command, canonical Run, head string) (Receipt, string, error) {
	id := AssignmentID(repo.ID, source.CommentID)
	receipt, err := s.repository.Receipt(ctx, repo, source, id)
	if err != nil {
		return Receipt{}, "", err
	}

	base := head
	disposition := "accepted"
	if receipt == nil && event.Attempt > 1 {
		// A missing/deleted receipt cannot prove whether a previous attempt ran.
		return Receipt{}, "", HistoryUnavailable
	}
	if receipt != nil {
		if receipt.AssignmentID != id || receipt.RunID != canonical.ID || receipt.RunAttempt <= 0 || receipt.RunAttempt > event.Attempt || receipt.ProfileID != command.ProfileID ||
			receipt.ProfileRevision != command.Revision || receipt.RequesterID != source.Requester.ID || !ValidSHA(receipt.BaseSHA) {
			return Receipt{}, "", HistoryUnavailable
		}

		ancestor, err := s.repository.Ancestor(ctx, repo, receipt.BaseSHA, head)
		if err != nil {
			return Receipt{}, "", err
		}
		if !ancestor {
			return Receipt{}, "", RevisionInvalid
		}

		base = receipt.BaseSHA
		disposition = "resume"
	}

	result := Receipt{AssignmentID: id, RunID: canonical.ID, RunAttempt: event.Attempt, BaseSHA: base, ProfileID: command.ProfileID, ProfileRevision: command.Revision, RequesterID: source.Requester.ID}
	if receipt != nil {
		result.CommentID = receipt.CommentID
	}
	return result, disposition, nil
}
