package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	gh "github.com/google/go-github/v74/github"
	"github.com/r59q/hub-rearranger/ops/agent-intake/internal/domain"
)

const receiptPrefix = "<!-- agent-intake:v1\n"

func (c *Client) Receipt(ctx context.Context, repo domain.Repository, source domain.Source, id string) (*domain.Receipt, error) {
	bot, _, err := c.api.Users.Get(ctx, "github-actions[bot]")
	if err != nil || bot.GetID() <= 0 || bot.GetType() != "Bot" {
		return nil, domain.Unavailable
	}

	options := &gh.IssueListCommentsOptions{ListOptions: gh.ListOptions{PerPage: 100, Page: 1}}
	var found *domain.Receipt
	for pages := 0; pages < 20; pages++ {
		comments, response, err := c.api.Issues.ListComments(ctx, repo.Owner, repo.Name, source.IssueNumber, options)
		if err != nil {
			return nil, safeError(err)
		}

		for _, comment := range comments {
			body := comment.GetBody()
			if comment.GetUser().GetID() != bot.GetID() || comment.GetUser().GetType() != "Bot" || !strings.HasPrefix(body, receiptPrefix) {
				continue
			}

			end := strings.Index(body, "\n-->")
			if end < len(receiptPrefix) || end > 2048 {
				return nil, domain.HistoryUnavailable
			}

			var receipt domain.Receipt
			decoder := json.NewDecoder(strings.NewReader(body[len(receiptPrefix):end]))
			decoder.DisallowUnknownFields()
			if decoder.Decode(&receipt) != nil || decoder.Decode(new(any)) != io.EOF {
				return nil, domain.HistoryUnavailable
			}

			if receipt.AssignmentID != id {
				continue
			}
			if found != nil || comment.GetID() <= 0 {
				return nil, domain.HistoryUnavailable
			}

			receipt.CommentID = comment.GetID()
			found = &receipt
		}

		if response.NextPage == 0 {
			return found, nil
		}
		options.Page = response.NextPage
	}
	return nil, domain.HistoryUnavailable
}

func (c *Client) Accept(ctx context.Context, repo domain.Repository, source domain.Source, receipt domain.Receipt, invocation domain.Invocation) (int64, error) {
	metadata, _ := json.Marshal(receipt)
	body := receiptPrefix + string(metadata) + "\n-->\n\n" +
		fmt.Sprintf("Agent assignment **accepted**: `%s`.\n\n", invocation.AssignmentID) +
		fmt.Sprintf("Source: [issue #%d](%s); [request comment](%s).\n\n", invocation.IssueNumber, invocation.SourceURL, invocation.RequestURL) +
		fmt.Sprintf("Profile: `%s@%s`; requester: `%s` (account %d); authority: `branch-draft-pr`.\n\n", invocation.ProfileID, invocation.ProfileRevision, invocation.Requester.Login, invocation.Requester.ID) +
		fmt.Sprintf("[Workflow attempt](%s). Execution awaits independent live authorization, runtime verification, and prior-attempt reconciliation. Intake acceptance does not prove runner readiness or publication.\n\n", invocation.RunURL) +
		"Rerun this original workflow after setup. Duplicate deliveries reuse this assignment; a changed profile policy requires a new request. This receipt does not grant authority."

	comment := &gh.IssueComment{Body: gh.Ptr(body)}
	var result *gh.IssueComment
	var err error
	if receipt.CommentID == 0 {
		result, _, err = c.api.Issues.CreateComment(ctx, repo.Owner, repo.Name, source.IssueNumber, comment)
	} else {
		result, _, err = c.api.Issues.EditComment(ctx, repo.Owner, repo.Name, receipt.CommentID, comment)
	}

	if err != nil {
		// A lost POST response may still have created the receipt. Reconcile once;
		// never issue a second POST blindly or dispatch after ambiguous publication.
		stored, readErr := c.Receipt(ctx, repo, source, receipt.AssignmentID)
		if readErr == nil && stored != nil && stored.RunID == receipt.RunID && stored.RunAttempt == receipt.RunAttempt && stored.BaseSHA == receipt.BaseSHA &&
			stored.ProfileID == receipt.ProfileID && stored.ProfileRevision == receipt.ProfileRevision && stored.RequesterID == receipt.RequesterID {
			return stored.CommentID, nil
		}
		return 0, domain.Unavailable
	}

	if result.GetID() <= 0 {
		return 0, domain.Unavailable
	}
	return result.GetID(), nil
}
