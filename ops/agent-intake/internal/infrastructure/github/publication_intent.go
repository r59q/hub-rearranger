package github

import (
	"context"
	"encoding/json"
	"io"
	"strings"

	gh "github.com/google/go-github/v74/github"
	"github.com/r59q/hub-rearranger/ops/agent-intake/internal/domain"
)

// publicationIntent updates the existing verified intake receipt, preserving its
// identity and prose. Intake carries this state forward across reruns. A started
// stage without its matching object never authorizes another non-idempotent POST.
func (c *Client) publicationIntent(ctx context.Context, input domain.Invocation, proposal domain.Proposal, value domain.Publication, stage string) error {
	repo := input.Repository
	bot, err := c.publicationBot(ctx)
	if err != nil {
		return err
	}
	comment, _, err := c.api.Issues.GetComment(ctx, repo.Owner, repo.Name, input.ReceiptCommentID)
	if err != nil {
		return domain.Unavailable
	}
	body := comment.GetBody()
	end := strings.Index(body, "\n-->")
	if comment.GetID() != input.ReceiptCommentID || comment.GetUser().GetID() != bot || comment.GetUser().GetType() != "Bot" || !strings.HasPrefix(body, receiptPrefix) || end < len(receiptPrefix) || end > 2048 {
		return domain.HistoryUnavailable
	}
	var receipt domain.Receipt
	decoder := json.NewDecoder(strings.NewReader(body[len(receiptPrefix):end]))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&receipt) != nil || decoder.Decode(new(any)) != io.EOF || receipt.AssignmentID != input.AssignmentID || receipt.RunID != input.RunID || receipt.RunAttempt != input.RunAttempt || receipt.BaseSHA != input.BaseSHA || receipt.ProfileID != input.ProfileID || receipt.ProfileRevision != input.ProfileRevision || receipt.RequesterID != input.Requester.ID {
		return domain.HistoryUnavailable
	}
	state := receipt.Publication
	if state == nil {
		state = &domain.PublicationIntent{ArtifactID: proposal.ArtifactID, Attempt: proposal.Attempt, HeadSHA: value.HeadSHA}
	}
	if state.ArtifactID != proposal.ArtifactID || state.Attempt != proposal.Attempt || state.HeadSHA != value.HeadSHA {
		return domain.HistoryUnavailable
	}
	switch stage {
	case "comment":
		if state.CommentStarted || state.CheckStarted {
			return domain.HistoryUnavailable
		}
		state.CommentStarted = true
	case "check":
		if !state.CommentStarted || state.CheckStarted {
			return domain.HistoryUnavailable
		}
		state.CheckStarted = true
	default:
		return domain.Invalid
	}
	receipt.Publication = state
	metadata, _ := json.Marshal(receipt)
	updated := receiptPrefix + string(metadata) + body[end:]
	result, _, err := c.api.Issues.EditComment(ctx, repo.Owner, repo.Name, input.ReceiptCommentID, &gh.IssueComment{Body: gh.Ptr(updated)})
	if err != nil || result.GetID() != input.ReceiptCommentID || result.GetBody() != updated || result.GetUser().GetID() != bot {
		return domain.HistoryUnavailable
	}
	// If this response is lost, no POST follows. A rerun finds started+missing and
	// fails closed instead of guessing whether the external write took place.
	return nil
}
