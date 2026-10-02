package github

import (
	"context"
	"fmt"

	gh "github.com/google/go-github/v74/github"
	"github.com/r59q/hub-rearranger/ops/agent-intake/internal/domain"
)

func (c *Client) publicationBot(ctx context.Context) (int64, error) {
	bot, _, err := c.api.Users.Get(ctx, "github-actions[bot]")
	if err != nil || bot.GetID() <= 0 || bot.GetType() != "Bot" || bot.GetLogin() != "github-actions[bot]" {
		return 0, domain.Unavailable
	}
	return bot.GetID(), nil
}

func (c *Client) verifyDraft(ctx context.Context, input domain.Invocation, proposal domain.Proposal, head string, pr *gh.PullRequest) (domain.Publication, error) {
	repo := input.Repository
	bot, err := c.publicationBot(ctx)
	if err != nil {
		return domain.Publication{}, err
	}
	if pr == nil {
		return domain.Publication{}, domain.HistoryUnavailable
	}
	ownerMatches := pr.GetUser().GetID() == bot && pr.GetUser().GetType() == "Bot"
	headMatches := pr.GetHead().GetRef() == domain.PublicationBranch(input) && pr.GetHead().GetSHA() == head && pr.GetHead().GetRepo().GetID() == repo.ID
	baseMatches := pr.GetBase().GetRepo().GetID() == repo.ID && pr.GetBase().GetRef() == repo.DefaultBranch && pr.GetBase().GetSHA() == input.BaseSHA
	metadataMatches := pr.GetBody() == draftBody(input, proposal, head) && pr.GetTitle() == draftTitle(input) && pr.GetHTMLURL() == fmt.Sprintf("%s/pull/%d", domain.RepositoryURL(repo), pr.GetNumber())
	stateMatches := pr.GetNumber() > 0 && pr.GetState() == "open" && pr.GetDraft() && !pr.GetMerged() && !pr.GetMaintainerCanModify()
	if !ownerMatches || !headMatches || !baseMatches || !metadataMatches || !stateMatches {
		return domain.Publication{}, domain.HistoryUnavailable
	}
	return domain.Publication{Branch: domain.PublicationBranch(input), HeadSHA: head, PRNumber: pr.GetNumber(), PRURL: pr.GetHTMLURL()}, nil
}

func (c *Client) findDraft(ctx context.Context, input domain.Invocation) (*gh.PullRequest, error) {
	repo := input.Repository
	var found *gh.PullRequest
	for page := 1; page <= 10; page++ {
		pulls, response, err := c.api.PullRequests.List(ctx, repo.Owner, repo.Name, &gh.PullRequestListOptions{State: "all", Head: repo.Owner + ":" + domain.PublicationBranch(input), ListOptions: gh.ListOptions{PerPage: 100, Page: page}})
		if err != nil {
			return nil, domain.Unavailable
		}
		for _, pr := range pulls {
			// Never assume the filter is itself ownership proof. A same-name fork PR
			// or duplicate/closed/moved PR must not authorize a replacement.
			if found != nil || pr.GetHead().GetRef() != domain.PublicationBranch(input) || pr.GetHead().GetRepo().GetID() != repo.ID {
				return nil, domain.HistoryUnavailable
			}
			found = pr
		}
		if response.NextPage == 0 {
			return found, nil
		}
	}
	return nil, domain.HistoryUnavailable
}

func (c *Client) EnsureDraft(ctx context.Context, input domain.Invocation, proposal domain.Proposal, head string) (domain.Publication, error) {
	exists, err := c.branch(ctx, input, head)
	if err != nil || !exists {
		return domain.Publication{}, domain.StaleHead
	}
	pr, err := c.findDraft(ctx, input)
	if err != nil {
		return domain.Publication{}, err
	}
	repo := input.Repository
	if pr == nil {
		_, _, createErr := c.api.PullRequests.Create(ctx, repo.Owner, repo.Name, &gh.NewPullRequest{Title: gh.Ptr(draftTitle(input)), Head: gh.Ptr(domain.PublicationBranch(input)), Base: gh.Ptr(repo.DefaultBranch), Body: gh.Ptr(draftBody(input, proposal, head)), Draft: gh.Ptr(true), MaintainerCanModify: gh.Ptr(false)})
		pr, err = c.findDraft(ctx, input)
		if err != nil {
			return domain.Publication{}, err
		}
		if pr == nil {
			if createErr != nil {
				return domain.Publication{}, domain.Unavailable
			}
			return domain.Publication{}, domain.HistoryUnavailable
		}
	}
	// List responses do not guarantee full mutable PR state. Re-read before use.
	pr, _, err = c.api.PullRequests.Get(ctx, repo.Owner, repo.Name, pr.GetNumber())
	if err != nil {
		return domain.Publication{}, domain.Unavailable
	}
	return c.verifyDraft(ctx, input, proposal, head, pr)
}

func (c *Client) ConfirmPublication(ctx context.Context, input domain.Invocation, proposal domain.Proposal, value domain.Publication) error {
	exists, err := c.branch(ctx, input, value.HeadSHA)
	if err != nil || !exists {
		return domain.StaleHead
	}
	pr, err := c.findDraft(ctx, input)
	if err != nil || pr == nil || pr.GetNumber() != value.PRNumber {
		return domain.HistoryUnavailable
	}
	stored, _, err := c.api.PullRequests.Get(ctx, input.Repository.Owner, input.Repository.Name, value.PRNumber)
	if err != nil {
		return domain.Unavailable
	}
	actual, err := c.verifyDraft(ctx, input, proposal, value.HeadSHA, stored)
	if err != nil {
		return err
	}
	if actual != value {
		return domain.HistoryUnavailable
	}
	return nil
}
