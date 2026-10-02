package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	gh "github.com/google/go-github/v74/github"
	"github.com/r59q/hub-rearranger/ops/agent-intake/internal/domain"
)

func (c *Client) executionJobs(ctx context.Context, repo domain.Repository, run *gh.WorkflowRun, attempt int) ([]*gh.WorkflowJob, error) {
	var result []*gh.WorkflowJob
	for page := 1; page <= 5; page++ {
		jobs, response, err := c.api.Actions.ListWorkflowJobsAttempt(ctx, repo.Owner, repo.Name, run.GetID(), int64(attempt), &gh.ListOptions{PerPage: 100, Page: page})
		if err != nil {
			return nil, domain.Unavailable
		}
		for _, job := range jobs.Jobs {
			if job.GetRunID() != run.GetID() || job.GetRunAttempt() != int64(attempt) || job.GetHeadSHA() != run.GetHeadSHA() {
				return nil, domain.HistoryUnavailable
			}
			result = append(result, job)
		}
		if response.NextPage == 0 {
			if len(result) != jobs.GetTotalCount() {
				return nil, domain.HistoryUnavailable
			}
			return result, nil
		}
	}
	return nil, domain.HistoryUnavailable
}

func uniqueJob(jobs []*gh.WorkflowJob, name string) (*gh.WorkflowJob, error) {
	var selected *gh.WorkflowJob
	for _, job := range jobs {
		if job.GetName() == name {
			if selected != nil {
				return nil, domain.HistoryUnavailable
			}
			selected = job
		}
	}
	return selected, nil
}

// ExecutionInput proves the original artifact came from this run's successful
// exact-attempt intake. Live authorization is separately reconstructed by domain.
func (c *Client) ExecutionInput(ctx context.Context, event domain.Event) (domain.Invocation, error) {
	// Actions embeds a reduced repository object without default_branch. Resolve
	// current repository policy through the repository endpoint instead.
	repo, err := c.Repository(ctx, event)
	if err != nil {
		return domain.Invocation{}, err
	}
	run, _, err := c.api.Actions.GetWorkflowRunByID(ctx, event.Repository.Owner, event.Repository.Name, event.RunID)
	if err != nil {
		return domain.Invocation{}, domain.Unavailable
	}
	_, valid := verifiedRun(run, event.Repository, event.CommentID)
	if !valid || repo.ID != event.Repository.ID || repo.DefaultBranch == "" || repo.Fork || repo.Archived || run.GetRunAttempt() != event.Attempt || run.GetHeadSHA() != event.WorkflowSHA || run.GetHeadRepository().GetID() != repo.ID || run.GetHeadBranch() != repo.DefaultBranch || (!repo.Private && (repo.Owner != "r59q" || repo.Name != "hub-rearranger")) {
		return domain.Invocation{}, domain.HistoryUnavailable
	}
	jobs, err := c.executionJobs(ctx, repo, run, event.Attempt)
	if err != nil {
		return domain.Invocation{}, err
	}
	job, err := uniqueJob(jobs, "authorize")
	if err != nil || job == nil || job.GetStatus() != "completed" || job.GetConclusion() != "success" {
		return domain.Invocation{}, domain.HistoryUnavailable
	}
	files, _, err := c.ExecutionArtifact(ctx, repo, run, fmt.Sprintf("agent-invocation-v1-%d", event.Attempt))
	if err != nil {
		return domain.Invocation{}, err
	}
	if len(files) != 1 || len(files["invocation.json"]) > 65536 {
		return domain.Invocation{}, domain.Invalid
	}
	var input domain.Invocation
	decoder := json.NewDecoder(bytes.NewReader(files["invocation.json"]))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&input) != nil || decoder.Decode(new(any)) != io.EOF {
		return domain.Invocation{}, domain.Invalid
	}
	return input, nil
}

// ReadinessRecord returns only origin-verified AW-006 evidence. Canonical schema
// and policy checks are performed by the trusted Python boundary before use.
func (c *Client) ReadinessRecord(ctx context.Context, input domain.Invocation) ([]byte, error) {
	repo := input.Repository
	workflow, _, err := c.api.Actions.GetWorkflowByFileName(ctx, repo.Owner, repo.Name, "agent-profile-diagnostic.yml")
	if err != nil || workflow.GetID() <= 0 || workflow.GetPath() != ".github/workflows/agent-profile-diagnostic.yml" || workflow.GetState() != "active" {
		return nil, domain.Failure("RUNNER_NOT_READY")
	}
	runs, _, err := c.api.Actions.ListWorkflowRunsByFileName(ctx, repo.Owner, repo.Name, "agent-profile-diagnostic.yml", &gh.ListWorkflowRunsOptions{Branch: repo.DefaultBranch, ListOptions: gh.ListOptions{PerPage: 1}})
	if err != nil || len(runs.WorkflowRuns) != 1 {
		return nil, domain.Failure("RUNNER_NOT_READY")
	}
	run := runs.WorkflowRuns[0]
	if run.GetWorkflowID() != workflow.GetID() || run.GetRepository().GetID() != repo.ID || run.GetHeadRepository().GetID() != repo.ID || run.GetRepository().GetFork() || (!run.GetRepository().GetPrivate() && (repo.Owner != "r59q" || repo.Name != "hub-rearranger")) || run.GetPath() != workflow.GetPath() || run.GetEvent() != "workflow_dispatch" || run.GetHeadBranch() != repo.DefaultBranch || run.GetHeadSHA() != input.PolicyRevision || run.GetStatus() != "completed" || run.GetRunAttempt() < 1 {
		return nil, domain.Failure("RUNNER_NOT_READY")
	}
	jobs, err := c.executionJobs(ctx, repo, run, run.GetRunAttempt())
	if err != nil {
		return nil, err
	}
	authorize, authErr := uniqueJob(jobs, "authorize")
	diagnostic, diagnosticErr := uniqueJob(jobs, "diagnostic")
	if authErr != nil || diagnosticErr != nil || authorize == nil || diagnostic == nil || authorize.GetConclusion() != "success" || authorize.GetStatus() != "completed" || diagnostic.GetConclusion() != "success" || diagnostic.GetStatus() != "completed" || diagnostic.GetRunnerID() <= 0 || diagnostic.GetRunnerName() == "" {
		return nil, domain.Failure("RUNNER_NOT_READY")
	}
	for _, label := range []string{"self-hosted", "linux", "hub-agent-codex"} {
		if !slices.Contains(diagnostic.Labels, label) {
			return nil, domain.Failure("RUNNER_NOT_READY")
		}
	}
	files, _, err := c.ExecutionArtifact(ctx, repo, run, fmt.Sprintf("agent-readiness-v1-%d", run.GetRunAttempt()))
	if err != nil || len(files) != 1 || files["readiness.json"] == nil {
		return nil, domain.Failure("RUNNER_NOT_READY")
	}
	// Timestamp binding is checked here against GitHub, not artifact assertions.
	var record struct {
		RunID      int64     `json:"run_id"`
		Attempt    int       `json:"run_attempt"`
		VerifiedAt time.Time `json:"verified_at"`
	}
	if json.Unmarshal(files["readiness.json"], &record) != nil || record.RunID != run.GetID() || record.Attempt != run.GetRunAttempt() || record.VerifiedAt.Before(diagnostic.GetStartedAt().Time) || record.VerifiedAt.After(diagnostic.GetCompletedAt().Time) {
		return nil, domain.Failure("RUNNER_NOT_READY")
	}
	return files["readiness.json"], nil
}

// PriorExecution reconciles every prior exact attempt. Once a runner started,
// missing/failed/expired output is ambiguous and never authorizes another run.
func (c *Client) PriorExecution(ctx context.Context, input domain.Invocation) (map[string][]byte, int64, error) {
	if input.RunAttempt > 100 {
		return nil, 0, domain.HistoryUnavailable
	}
	var recovered map[string][]byte
	var artifactID int64
	for attempt := 1; attempt < input.RunAttempt; attempt++ {
		run, _, err := c.api.Actions.GetWorkflowRunAttempt(ctx, input.Repository.Owner, input.Repository.Name, input.RunID, attempt, nil)
		if err != nil {
			return nil, 0, domain.HistoryUnavailable
		}
		verified, valid := verifiedRun(run, input.Repository, input.RequestCommentID)
		if !valid || verified.ID != input.RunID || verified.Attempt != attempt {
			return nil, 0, domain.HistoryUnavailable
		}
		jobs, err := c.executionJobs(ctx, input.Repository, run, attempt)
		if err != nil {
			return nil, 0, err
		}
		patch, err := uniqueJob(jobs, "patch")
		if err != nil || patch == nil {
			// Only the known AW-011 execution-disabled job proves hosted-only
			// history. An absent job in arbitrary/truncated history proves nothing.
			legacy, legacyErr := uniqueJob(jobs, "Execution disabled (AW-012 required)")
			if strings.Contains(run.GetPath(), "@") || err != nil || legacyErr != nil || legacy == nil || legacy.GetStatus() != "completed" || legacy.GetConclusion() != "success" || slices.Contains(legacy.Labels, "self-hosted") {
				return nil, 0, domain.HistoryUnavailable
			}
			continue
		}
		if patch.GetConclusion() == "skipped" && patch.GetRunnerID() == 0 {
			continue
		}
		if patch.GetStatus() != "completed" || patch.GetConclusion() != "success" || patch.GetRunnerID() <= 0 || !slices.Contains(patch.Labels, "hub-agent-codex") || recovered != nil {
			return nil, 0, domain.HistoryUnavailable
		}
		authorize, err := uniqueJob(jobs, "authorize")
		if err != nil || authorize == nil || authorize.GetStatus() != "completed" || authorize.GetConclusion() != "success" {
			return nil, 0, domain.HistoryUnavailable
		}
		recovered, artifactID, err = c.ExecutionArtifact(ctx, input.Repository, run, fmt.Sprintf("agent-patch-v1-%d", attempt))
		if err != nil {
			return nil, 0, err
		}
		var identity struct {
			Attempt int `json:"run_attempt"`
		}
		if json.Unmarshal(recovered["result.json"], &identity) != nil || identity.Attempt != attempt {
			return nil, 0, domain.HistoryUnavailable
		}
	}
	return recovered, artifactID, nil
}
