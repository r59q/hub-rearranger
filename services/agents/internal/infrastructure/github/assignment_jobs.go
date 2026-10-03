package github

import (
	"context"

	gh "github.com/google/go-github/v74/github"
	"github.com/r59q/hub-rearranger/services/agents/internal/domain"
)

func (r *AssignmentReader) assignmentJobState(ctx context.Context, repo domain.Repository, run *gh.WorkflowRun) (string, string, error) {
	jobs, err := r.assignmentJobs(ctx, repo, run)
	if err != nil {
		return "", "", err
	}
	if run.GetConclusion() != "success" {
		return "", "", nil
	}
	patch := jobs["patch"]
	if patch == nil {
		return "", "", domain.ErrGitHubUnavailable
	}
	if patch.GetConclusion() == "skipped" {
		return "blocked", "The patch job was skipped. Check the original workflow summary for authorization, runner setup, or recovered proposal evidence.", nil
	}
	return "", "", nil
}

func (r *AssignmentReader) assignmentJobs(ctx context.Context, repo domain.Repository, run *gh.WorkflowRun) (map[string]*gh.WorkflowJob, error) {
	found := map[string]*gh.WorkflowJob{}
	for page := 1; page <= 10; page++ {
		list, response, err := r.client.Actions.ListWorkflowJobsAttempt(ctx, repo.Owner, repo.Name, run.GetID(), int64(run.GetRunAttempt()), &gh.ListOptions{PerPage: 100, Page: page})
		if err != nil {
			return nil, classify(err, response)
		}
		for _, job := range list.Jobs {
			switch job.GetName() {
			case "authorize", "dispatch", "patch", "publish":
			default:
				continue
			}
			if found[job.GetName()] != nil || job.GetRunID() != run.GetID() || job.GetHeadSHA() != run.GetHeadSHA() {
				return nil, domain.ErrGitHubUnavailable
			}
			found[job.GetName()] = job
		}
		if response.NextPage == 0 {
			return found, nil
		}
	}
	return nil, domain.ErrGitHubUnavailable
}

func (r *AssignmentReader) verifyProposalJobs(ctx context.Context, repo domain.Repository, run *gh.WorkflowRun) error {
	jobs, err := r.assignmentJobs(ctx, repo, run)
	if err != nil {
		return err
	}
	for _, name := range []string{"authorize", "dispatch", "patch"} {
		job := jobs[name]
		if job == nil || job.GetStatus() != "completed" || job.GetConclusion() != "success" {
			return domain.ErrGitHubUnavailable
		}
	}
	patch := jobs["patch"]
	if patch.GetRunnerID() <= 0 {
		return domain.ErrGitHubUnavailable
	}
	labels := map[string]bool{}
	for _, label := range patch.Labels {
		labels[label] = true
	}
	if !labels["self-hosted"] || !labels["linux"] || !labels["hub-agent-codex"] {
		return domain.ErrGitHubUnavailable
	}
	return nil
}
