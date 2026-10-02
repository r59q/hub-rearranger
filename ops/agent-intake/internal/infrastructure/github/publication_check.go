package github

import (
	"context"
	"fmt"
	"time"

	gh "github.com/google/go-github/v74/github"
	"github.com/r59q/hub-rearranger/ops/agent-intake/internal/domain"
)

const publicationCheckName = "Agent proposal / repository-check"

func checkConclusion(proposal domain.Proposal) string {
	if validationOutcome(proposal) == "passed" {
		return "success"
	}
	return "neutral"
}

func checkSummary(input domain.Invocation, proposal domain.Proposal, value domain.Publication) string {
	return fmt.Sprintf("Draft patch proposal: [PR #%d](%s). Repository validation: **%s**.\n\n[Assignment request](%s); [verified proposal attempt](%s). Human review is required.", value.PRNumber, value.PRURL, validationOutcome(proposal), input.RequestURL, proposal.Invocation.RunURL)
}

func (c *Client) findPublicationCheck(ctx context.Context, input domain.Invocation, proposal domain.Proposal, value domain.Publication) (bool, error) {
	repo := input.Repository
	app, _, err := c.api.Apps.Get(ctx, "github-actions")
	if err != nil || app.GetID() <= 0 || app.GetSlug() != "github-actions" {
		return false, domain.Unavailable
	}
	found := false
	for page := 1; page <= 10; page++ {
		checks, response, err := c.api.Checks.ListCheckRunsForRef(ctx, repo.Owner, repo.Name, value.HeadSHA, &gh.ListCheckRunsOptions{CheckName: gh.Ptr(publicationCheckName), Filter: gh.Ptr("all"), ListOptions: gh.ListOptions{PerPage: 100, Page: page}})
		if err != nil {
			return false, domain.Unavailable
		}
		for _, check := range checks.CheckRuns {
			if check.GetExternalID() != "agent-publication:"+input.AssignmentID {
				continue
			}
			if found || check.GetID() <= 0 || check.GetApp().GetID() != app.GetID() || check.GetName() != publicationCheckName || check.GetHeadSHA() != value.HeadSHA || check.GetDetailsURL() != proposal.Invocation.RunURL || check.GetStatus() != "completed" || check.GetConclusion() != checkConclusion(proposal) || check.GetOutput().GetSummary() != checkSummary(input, proposal, value) {
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

func (c *Client) EnsurePublicationCheck(ctx context.Context, input domain.Invocation, proposal domain.Proposal, value domain.Publication) error {
	if err := c.ConfirmPublication(ctx, input, proposal, value); err != nil {
		return err
	}
	exists, err := c.findPublicationCheck(ctx, input, proposal, value)
	if err != nil || exists {
		return err
	}
	if err = c.publicationIntent(ctx, input, proposal, value, "check"); err != nil {
		return err
	}
	if err = c.ConfirmPublication(ctx, input, proposal, value); err != nil {
		return err
	}
	_, _, createErr := c.api.Checks.CreateCheckRun(ctx, input.Repository.Owner, input.Repository.Name, gh.CreateCheckRunOptions{Name: publicationCheckName, HeadSHA: value.HeadSHA, DetailsURL: gh.Ptr(proposal.Invocation.RunURL), ExternalID: gh.Ptr("agent-publication:" + input.AssignmentID), Status: gh.Ptr("completed"), Conclusion: gh.Ptr(checkConclusion(proposal)), CompletedAt: &gh.Timestamp{Time: time.Now().UTC()}, Output: &gh.CheckRunOutput{Title: gh.Ptr("Draft proposal; repository validation " + validationOutcome(proposal)), Summary: gh.Ptr(checkSummary(input, proposal, value))}})
	exists, err = c.findPublicationCheck(ctx, input, proposal, value)
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
