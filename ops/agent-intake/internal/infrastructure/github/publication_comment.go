package github

import (
	"context"
	"encoding/json"
	"io"
	"strings"

	gh "github.com/google/go-github/v74/github"
	"github.com/r59q/hub-rearranger/ops/agent-intake/internal/domain"
)

func (c *Client) findPublicationComment(ctx context.Context, input domain.Invocation, proposal domain.Proposal, value domain.Publication) (bool, error) {
	bot, err := c.publicationBot(ctx)
	if err != nil {
		return false, err
	}
	expected := publicationComment(input, proposal, value)
	found := false
	for page := 1; page <= 20; page++ {
		comments, response, err := c.api.Issues.ListComments(ctx, input.Repository.Owner, input.Repository.Name, input.IssueNumber, &gh.IssueListCommentsOptions{ListOptions: gh.ListOptions{PerPage: 100, Page: page}})
		if err != nil {
			return false, domain.Unavailable
		}
		for _, comment := range comments {
			body := comment.GetBody()
			if comment.GetUser().GetID() != bot || comment.GetUser().GetType() != "Bot" || !strings.HasPrefix(body, "<!-- agent-assignment:v1\n") {
				continue
			}
			end := strings.Index(body, "\n-->")
			const prefix = "<!-- agent-assignment:v1\n"
			if end < len(prefix) || end > 4096 {
				return false, domain.HistoryUnavailable
			}
			var metadata assignmentProvenance
			decoder := json.NewDecoder(strings.NewReader(body[len(prefix):end]))
			decoder.DisallowUnknownFields()
			if decoder.Decode(&metadata) != nil || decoder.Decode(new(any)) != io.EOF {
				return false, domain.HistoryUnavailable
			}
			if metadata.RequestCommentID != input.RequestCommentID {
				continue
			}
			if found || comment.GetID() <= 0 || body != expected {
				return false, domain.HistoryUnavailable
			}
			found = true
		}
		if response.NextPage == 0 {
			return found, nil
		}
	}
	return false, domain.HistoryUnavailable
}

func (c *Client) EnsurePublicationComment(ctx context.Context, input domain.Invocation, proposal domain.Proposal, value domain.Publication) error {
	if err := c.ConfirmPublication(ctx, input, proposal, value); err != nil {
		return err
	}
	exists, err := c.findPublicationComment(ctx, input, proposal, value)
	if err != nil || exists {
		return err
	}
	if err = c.publicationIntent(ctx, input, proposal, value, "comment"); err != nil {
		return err
	}
	if err = c.ConfirmPublication(ctx, input, proposal, value); err != nil {
		return err
	}
	_, _, createErr := c.api.Issues.CreateComment(ctx, input.Repository.Owner, input.Repository.Name, input.IssueNumber, &gh.IssueComment{Body: gh.Ptr(publicationComment(input, proposal, value))})
	exists, err = c.findPublicationComment(ctx, input, proposal, value)
	if err != nil {
		return err
	}
	if !exists {
		if createErr != nil {
			return domain.Unavailable
		}
		return domain.HistoryUnavailable
	}
	return nil
}
