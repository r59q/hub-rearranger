package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"slices"

	"github.com/r59q/hub-rearranger/ops/agent-intake/internal/domain"
)

func decodeInvocation(content []byte) (domain.Invocation, error) {
	var input domain.Invocation
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	if len(content) == 0 || len(content) > 65536 || decoder.Decode(&input) != nil || decoder.Decode(new(any)) != io.EOF {
		return input, domain.Invalid
	}
	return input, nil
}

// Proposal selects an exact successful attempt using GitHub jobs, not job
// outputs/artifact assertions. A recovered proposal never starts Codex again.
func (c *Client) Proposal(ctx context.Context, input domain.Invocation) (domain.Proposal, error) {
	prior, _, err := c.PriorExecution(ctx, input)
	if err != nil {
		return domain.Proposal{}, err
	}
	run, _, err := c.api.Actions.GetWorkflowRunAttempt(ctx, input.Repository.Owner, input.Repository.Name, input.RunID, input.RunAttempt, nil)
	if err != nil {
		return domain.Proposal{}, domain.HistoryUnavailable
	}
	jobs, err := c.executionJobs(ctx, input.Repository, run, input.RunAttempt)
	if err != nil {
		return domain.Proposal{}, err
	}
	patch, err := uniqueJob(jobs, "patch")
	if err != nil || patch == nil {
		return domain.Proposal{}, domain.HistoryUnavailable
	}
	attempt := input.RunAttempt
	if patch.GetConclusion() == "skipped" && patch.GetRunnerID() == 0 && prior != nil {
		var identity struct {
			Attempt int `json:"run_attempt"`
		}
		if json.Unmarshal(prior["result.json"], &identity) != nil || identity.Attempt < 1 || identity.Attempt >= input.RunAttempt {
			return domain.Proposal{}, domain.HistoryUnavailable
		}
		attempt = identity.Attempt
	} else if prior != nil {
		return domain.Proposal{}, domain.HistoryUnavailable
	}
	return c.proposalAttempt(ctx, input, attempt)
}

func (c *Client) proposalAttempt(ctx context.Context, input domain.Invocation, attempt int) (domain.Proposal, error) {
	repo := input.Repository
	run, _, err := c.api.Actions.GetWorkflowRunAttempt(ctx, repo.Owner, repo.Name, input.RunID, attempt, nil)
	if err != nil {
		return domain.Proposal{}, domain.HistoryUnavailable
	}
	verified, valid := verifiedRun(run, repo, input.RequestCommentID)
	if !valid || verified.ID != input.RunID || verified.Attempt != attempt || run.GetHeadRepository().GetID() != repo.ID || run.GetHeadBranch() != repo.DefaultBranch || run.GetCreatedAt().Time.IsZero() {
		return domain.Proposal{}, domain.HistoryUnavailable
	}
	jobs, err := c.executionJobs(ctx, repo, run, attempt)
	if err != nil {
		return domain.Proposal{}, err
	}
	for _, name := range []string{"authorize", "dispatch", "patch"} {
		job, err := uniqueJob(jobs, name)
		if err != nil || job == nil || job.GetStatus() != "completed" || job.GetConclusion() != "success" {
			return domain.Proposal{}, domain.HistoryUnavailable
		}
		if name == "patch" {
			if job.GetRunnerID() <= 0 || job.GetRunnerName() == "" {
				return domain.Proposal{}, domain.HistoryUnavailable
			}
			for _, label := range []string{"self-hosted", "linux", "hub-agent-codex"} {
				if !slices.Contains(job.Labels, label) {
					return domain.Proposal{}, domain.HistoryUnavailable
				}
			}
		}
	}
	invocation, _, err := c.ExecutionArtifact(ctx, repo, run, fmt.Sprintf("agent-invocation-v1-%d", attempt))
	if err != nil || len(invocation) != 1 {
		return domain.Proposal{}, domain.HistoryUnavailable
	}
	origin, err := decodeInvocation(invocation["invocation.json"])
	if err != nil {
		return domain.Proposal{}, err
	}
	expected := input
	expected.RunAttempt, expected.RunURL = attempt, fmt.Sprintf("%s/actions/runs/%d/attempts/%d", domain.RepositoryURL(repo), input.RunID, attempt)
	expected.PolicyRevision, expected.Profile, expected.RequesterRole = origin.PolicyRevision, origin.Profile, origin.RequesterRole
	if !reflect.DeepEqual(origin, expected) || !domain.ValidSHA(origin.PolicyRevision) || (origin.RequesterRole != "maintain" && origin.RequesterRole != "admin") {
		return domain.Proposal{}, domain.HistoryUnavailable
	}
	ancestor, err := c.Ancestor(ctx, repo, origin.PolicyRevision, input.PolicyRevision)
	if err != nil || !ancestor {
		return domain.Proposal{}, domain.HistoryUnavailable
	}
	files, id, err := c.ExecutionArtifact(ctx, repo, run, fmt.Sprintf("agent-patch-v1-%d", attempt))
	if err != nil {
		return domain.Proposal{}, err
	}
	if len(files) != 3 || len(files["result.json"]) == 0 || len(files["result.json"]) > 65536 || len(files["proposal.patch"]) == 0 || len(files["summary.json"]) == 0 {
		return domain.Proposal{}, domain.HistoryUnavailable
	}
	return domain.Proposal{Invocation: origin, ArtifactID: id, Attempt: attempt, Created: run.GetCreatedAt().Time, Result: files["result.json"], Patch: files["proposal.patch"], Summary: files["summary.json"]}, nil
}
