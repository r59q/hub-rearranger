package github

import (
	"context"
	"path"

	gh "github.com/google/go-github/v74/github"
	"github.com/r59q/hub-rearranger/services/agents/internal/domain"
)

func (r *ReadinessReader) latestEvidence(ctx context.Context, repo domain.Repository, branch string, workflowID int64) (*domain.EvidenceOrigin, *domain.RuntimeEvidence, error) {
	// Never filter to successful runs or a head SHA: a newer pending/failed run
	// must supersede an older green result, including reruns of the same run ID.
	runs, response, err := r.client.Actions.ListWorkflowRunsByFileName(ctx, repo.Owner, repo.Name, path.Base(domain.DiagnosticWorkflowPath), &gh.ListWorkflowRunsOptions{Branch: branch, ListOptions: gh.ListOptions{PerPage: 1}})
	if err != nil {
		return nil, nil, classify(err, response)
	}
	if len(runs.WorkflowRuns) == 0 {
		return nil, nil, nil
	}

	run := runs.WorkflowRuns[0]
	origin := &domain.EvidenceOrigin{Repository: run.GetRepository().GetFullName(), Revision: run.GetHeadSHA(), Branch: run.GetHeadBranch(), WorkflowPath: run.GetPath(), Event: run.GetEvent(), RunID: run.GetID(), RunAttempt: run.GetRunAttempt(), Completed: run.GetStatus() == "completed", StartedAt: run.GetRunStartedAt().Time, CompletedAt: run.GetUpdatedAt().Time}
	if run.GetWorkflowID() != workflowID || origin.RunID <= 0 || run.GetRepository().GetID() <= 0 || !origin.Completed || origin.RunAttempt < 1 || run.GetHeadRepository().GetID() != run.GetRepository().GetID() || run.GetRepository().GetFork() {
		return origin, nil, nil
	}

	// Read exact attempt jobs, paginate with a strict bound. Name ambiguity or an
	// incomplete page is never trusted. Github metadata, not evidence, owns labels.
	authorized, diagnostics := 0, 0
	for page := 1; page <= 5; page++ {
		jobs, response, err := r.client.Actions.ListWorkflowJobsAttempt(ctx, repo.Owner, repo.Name, origin.RunID, int64(origin.RunAttempt), &gh.ListOptions{PerPage: 100, Page: page})
		if err != nil {
			return nil, nil, classify(err, response)
		}

		for _, job := range jobs.Jobs {
			if job.GetRunID() != origin.RunID || job.GetRunAttempt() != int64(origin.RunAttempt) || job.GetHeadSHA() != origin.Revision {
				continue
			}
			switch job.GetName() {
			case "authorize":
				authorized++
				origin.Authorized = job.GetStatus() == "completed" && job.GetConclusion() == "success"
			case "diagnostic":
				diagnostics++
				origin.DiagnosticSucceeded = job.GetStatus() == "completed" && job.GetConclusion() == "success" && job.GetRunnerID() > 0 && job.GetRunnerName() != ""
				origin.RunnerLabels = job.Labels
				origin.StartedAt = job.GetStartedAt().Time
				origin.CompletedAt = job.GetCompletedAt().Time
			}
		}

		if response.NextPage == 0 {
			break
		}
		if page == 5 {
			return origin, nil, nil
		}
	}

	if authorized != 1 || diagnostics != 1 || !origin.Authorized || !origin.DiagnosticSucceeded {
		return origin, nil, nil
	}

	record, err := r.readArtifact(ctx, repo, run)
	return origin, record, err
}
