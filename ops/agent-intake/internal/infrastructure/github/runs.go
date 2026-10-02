package github

import (
	"context"
	"strings"
	"time"

	gh "github.com/google/go-github/v74/github"
	"github.com/r59q/hub-rearranger/ops/agent-intake/internal/domain"
)

func verifiedRun(run *gh.WorkflowRun, repo domain.Repository, commentID int64) (domain.Run, bool) {
	path := strings.SplitN(run.GetPath(), "@", 2)[0]
	result := domain.Run{ID: run.GetID(), Attempt: run.GetRunAttempt(), HeadSHA: run.GetHeadSHA(), Path: path,
		Event: run.GetEvent(), Title: run.GetDisplayTitle(), RepositoryID: run.GetRepository().GetID()}
	valid := result.ID > 0 && result.RepositoryID == repo.ID && result.Event == "issue_comment" &&
		result.Path == domain.WorkflowPath && result.Title == domain.RunTitle(repo.ID, commentID) && domain.ValidSHA(result.HeadSHA)
	return result, valid
}

// CanonicalRun elects the earliest GitHub run for this immutable comment. Only
// that run may dispatch; a duplicate directs the user to rerun the original.
// A bounded/incomplete history fails closed instead of selecting a new owner.
func (c *Client) CanonicalRun(ctx context.Context, repo domain.Repository, event domain.Event, source domain.Source) (domain.Run, error) {
	current, _, err := c.api.Actions.GetWorkflowRunByID(ctx, repo.Owner, repo.Name, event.RunID)
	if err != nil {
		return domain.Run{}, safeError(err)
	}
	canonical, valid := verifiedRun(current, repo, source.CommentID)
	if !valid || canonical.Attempt != event.Attempt || canonical.HeadSHA != event.WorkflowSHA ||
		current.GetActor().GetID() != source.Requester.ID || !strings.EqualFold(current.GetActor().GetLogin(), event.Actor) ||
		!strings.EqualFold(current.GetTriggeringActor().GetLogin(), event.RerunActor) {
		return domain.Run{}, domain.HistoryUnavailable
	}
	if current.GetCreatedAt().Time.IsZero() || current.GetCreatedAt().Time.Before(source.Created.Add(-time.Second)) {
		return domain.Run{}, domain.HistoryUnavailable
	}

	// Bound the historical window by this run's original creation, even on rerun.
	window := source.Created.Add(-time.Second).UTC().Format(time.RFC3339) + ".." + current.GetCreatedAt().Add(time.Second).UTC().Format(time.RFC3339)
	options := &gh.ListWorkflowRunsOptions{Event: "issue_comment", Created: window,
		ListOptions: gh.ListOptions{PerPage: 100, Page: 1}}
	seenCurrent := false
	for pages := 0; pages < 10; pages++ {
		runs, response, err := c.api.Actions.ListWorkflowRunsByFileName(ctx, repo.Owner, repo.Name, "agent-assignment.yml", options)
		if err != nil {
			return domain.Run{}, safeError(err)
		}
		// GitHub caps filtered history at 1000 results. Do not silently lose an older owner.
		if runs.GetTotalCount() > 1000 {
			return domain.Run{}, domain.HistoryUnavailable
		}

		for _, run := range runs.WorkflowRuns {
			candidate, valid := verifiedRun(run, repo, source.CommentID)
			if !valid {
				continue
			}
			if candidate.ID == event.RunID {
				seenCurrent = true
			}
			if candidate.ID < canonical.ID {
				canonical = candidate
			}
		}

		if response.NextPage == 0 {
			if !seenCurrent {
				return domain.Run{}, domain.HistoryUnavailable
			}
			return canonical, nil
		}
		options.Page = response.NextPage
	}
	return domain.Run{}, domain.HistoryUnavailable
}
