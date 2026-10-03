package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	gh "github.com/google/go-github/v74/github"
	"github.com/r59q/hub-rearranger/services/agents/internal/domain"
	"github.com/r59q/hub-rearranger/services/agents/internal/infrastructure/profiles"
)

type assignmentMetadata struct {
	Source           string `json:"source"`
	RequestCommentID int64  `json:"request_comment_id"`
	Request          string `json:"request"`
	ProfileID        string `json:"profile_id"`
	ProfileRevision  string `json:"profile_revision"`
	RequesterID      int64  `json:"requester_id"`
	Requester        string `json:"requester"`
	Authority        string `json:"authority"`
	Run              string `json:"run"`
}

type publicationMetadata struct {
	Version      int    `json:"version"`
	AssignmentID string `json:"assignment_id"`
	BaseSHA      string `json:"base_sha"`
	HeadSHA      string `json:"head_sha"`
	ArtifactID   int64  `json:"artifact_id"`
	Attempt      int    `json:"proposal_attempt"`
	PatchSHA256  string `json:"patch_sha256"`
}

func marker(body, name string, value any) bool {
	prefix := "<!-- " + name + ":v1\n"
	if strings.Count(body, prefix) != 1 {
		return false
	}
	start := strings.Index(body, prefix) + len(prefix)
	end := strings.Index(body[start:], "\n-->")
	if end < 0 || end > 4096 {
		return false
	}
	_, valid := profiles.ParseDocument([]byte(body[start : start+end]))
	if !valid {
		return false
	}
	decoder := json.NewDecoder(strings.NewReader(body[start : start+end]))
	decoder.DisallowUnknownFields()
	return decoder.Decode(value) == nil && decoder.Decode(new(any)) == io.EOF
}

func (r *AssignmentReader) assignmentProposal(ctx context.Context, repo domain.Repository, repository *gh.Repository, request *domain.AssignmentRequest, run *gh.WorkflowRun) (*domain.AssignmentProposal, error) {
	branch := fmt.Sprintf("agent/codex-thorough/%d-%d", repository.GetID(), request.CommentID)
	pulls, response, err := r.client.PullRequests.List(ctx, repo.Owner, repo.Name, &gh.PullRequestListOptions{State: "all", Head: repo.Owner + ":" + branch, ListOptions: gh.ListOptions{PerPage: 100}})
	if err != nil {
		return nil, classify(err, response)
	}
	if len(pulls) == 0 && response.NextPage == 0 {
		return nil, nil
	}
	if len(pulls) != 1 || response.NextPage != 0 {
		return nil, domain.ErrGitHubUnavailable
	}
	pr, response, err := r.client.PullRequests.Get(ctx, repo.Owner, repo.Name, pulls[0].GetNumber())
	if err != nil {
		return nil, classify(err, response)
	}
	bot, response, err := r.client.Users.Get(ctx, "github-actions[bot]")
	if err != nil {
		return nil, classify(err, response)
	}
	var assignment assignmentMetadata
	var publication publicationMetadata
	if !marker(pr.GetBody(), "agent-assignment", &assignment) || !marker(pr.GetBody(), "agent-publication", &publication) {
		return nil, domain.ErrGitHubUnavailable
	}
	expected := assignmentMetadata{Source: fmt.Sprintf("https://github.com/%s/issues/%d", repo.FullName(), requestIssue(request)), RequestCommentID: request.CommentID, Request: request.URL, ProfileID: "codex-thorough", ProfileRevision: request.ProfileRevision, RequesterID: request.RequesterID, Requester: request.Requester, Authority: "branch-draft-pr", Run: fmt.Sprintf("https://github.com/%s/actions/runs/%d/attempts/%d", repo.FullName(), run.GetID(), publication.Attempt)}
	id := fmt.Sprintf("%d:%d", repository.GetID(), request.CommentID)
	if assignment != expected || publication.Version != 1 || publication.AssignmentID != id || publication.Attempt < 1 || publication.Attempt > run.GetRunAttempt() || publication.ArtifactID <= 0 || !commitSHA.MatchString(publication.BaseSHA) || !commitSHA.MatchString(publication.HeadSHA) {
		return nil, domain.ErrGitHubUnavailable
	}
	if bot.GetID() <= 0 || bot.GetType() != "Bot" || pr.GetUser().GetID() != bot.GetID() || pr.GetUser().GetType() != "Bot" || pr.GetNumber() <= 0 || pr.GetHTMLURL() != fmt.Sprintf("https://github.com/%s/pull/%d", repo.FullName(), pr.GetNumber()) || pr.GetHead().GetRef() != branch || pr.GetHead().GetSHA() != publication.HeadSHA || pr.GetHead().GetRepo().GetID() != repository.GetID() || pr.GetBase().GetRepo().GetID() != repository.GetID() || pr.GetBase().GetRef() != repository.GetDefaultBranch() {
		return nil, domain.ErrGitHubUnavailable
	}
	ref, response, err := r.client.Git.GetRef(ctx, repo.Owner, repo.Name, "heads/"+branch)
	if err != nil {
		return nil, classify(err, response)
	}
	if ref.GetObject().GetSHA() != publication.HeadSHA || ref.GetObject().GetType() != "commit" {
		return nil, domain.ErrGitHubUnavailable
	}
	attempt, response, err := r.client.Actions.GetWorkflowRunAttempt(ctx, repo.Owner, repo.Name, run.GetID(), publication.Attempt, nil)
	if err != nil {
		return nil, classify(err, response)
	}
	if !validAssignmentRun(attempt, repository.GetID(), *request) || attempt.GetRunAttempt() != publication.Attempt || attempt.GetHeadBranch() != repository.GetDefaultBranch() {
		return nil, domain.ErrGitHubUnavailable
	}
	validation, err := r.verifyProposalArtifact(ctx, repo, repository, request, attempt, publication)
	if err != nil {
		return nil, err
	}
	commit, response, err := r.client.Git.GetCommit(ctx, repo.Owner, repo.Name, publication.HeadSHA)
	if err != nil {
		return nil, classify(err, response)
	}
	message := fmt.Sprintf("Propose issue #%d for agent assignment %s\n\nProfile: codex-thorough@%s\nBase: %s\nProposal: %d/%d artifact %d\nPatch-SHA256: %s", requestIssue(request), id, request.ProfileRevision, publication.BaseSHA, run.GetID(), publication.Attempt, publication.ArtifactID, publication.PatchSHA256)
	if commit.GetSHA() != publication.HeadSHA || len(commit.Parents) != 1 || commit.Parents[0].GetSHA() != publication.BaseSHA || commit.GetMessage() != message || !commitSHA.MatchString(commit.GetTree().GetSHA()) || !proposalAuthor(commit.GetAuthor(), attempt) || !proposalAuthor(commit.GetCommitter(), attempt) {
		return nil, domain.ErrGitHubUnavailable
	}
	proposal := &domain.AssignmentProposal{Number: int64(pr.GetNumber()), URL: pr.GetHTMLURL(), State: pr.GetState(), Draft: pr.GetDraft(), Branch: branch, BranchURL: fmt.Sprintf("https://github.com/%s/tree/%s", repo.FullName(), branch), HeadSHA: publication.HeadSHA}
	check, err := r.assignmentCheck(ctx, repo, request, proposal, assignment.Run, id, validation)
	if err != nil {
		return nil, err
	}
	proposal.Check = check
	return proposal, nil
}

func requestIssue(request *domain.AssignmentRequest) int { return int(request.IssueNumber) }

func proposalAuthor(author *gh.CommitAuthor, run *gh.WorkflowRun) bool {
	return author.GetName() == "github-actions[bot]" && author.GetEmail() == "41898282+github-actions[bot]@users.noreply.github.com" && author.GetDate().Time.Equal(run.GetCreatedAt().Time)
}

func (r *AssignmentReader) assignmentCheck(ctx context.Context, repo domain.Repository, request *domain.AssignmentRequest, proposal *domain.AssignmentProposal, runURL, id, validation string) (*domain.AssignmentCheck, error) {
	app, response, err := r.client.Apps.Get(ctx, "github-actions")
	if err != nil {
		return nil, classify(err, response)
	}
	if app.GetID() <= 0 || app.GetSlug() != "github-actions" {
		return nil, domain.ErrGitHubUnavailable
	}
	var found *domain.AssignmentCheck
	expected := fmt.Sprintf("Draft patch proposal: [PR #%d](%s). Repository validation: **%s**.\n\n[Assignment request](%s); [verified proposal attempt](%s). Human review is required.", proposal.Number, proposal.URL, validation, request.URL, runURL)
	conclusion := "neutral"
	if validation == "passed" {
		conclusion = "success"
	}
	for page := 1; page <= 10; page++ {
		checks, response, err := r.client.Checks.ListCheckRunsForRef(ctx, repo.Owner, repo.Name, proposal.HeadSHA, &gh.ListCheckRunsOptions{CheckName: gh.Ptr("Agent proposal / repository-check"), Filter: gh.Ptr("all"), ListOptions: gh.ListOptions{PerPage: 100, Page: page}})
		if err != nil {
			return nil, classify(err, response)
		}
		for _, check := range checks.CheckRuns {
			if check.GetExternalID() != "agent-publication:"+id {
				continue
			}
			url := fmt.Sprintf("https://github.com/%s/runs/%d", repo.FullName(), check.GetID())
			if found != nil || check.GetID() <= 0 || check.GetApp().GetID() != app.GetID() || check.GetHeadSHA() != proposal.HeadSHA || check.GetName() != "Agent proposal / repository-check" || check.GetStatus() != "completed" || check.GetConclusion() != conclusion || check.GetOutput().GetSummary() != expected || (check.GetDetailsURL() != url && check.GetDetailsURL() != runURL) {
				return nil, domain.ErrGitHubUnavailable
			}
			found = &domain.AssignmentCheck{URL: url, Status: check.GetStatus(), Conclusion: check.GetConclusion(), Summary: "Repository validation: " + validation + ". Human review is required."}
		}
		if response.NextPage == 0 {
			return found, nil
		}
	}
	return nil, domain.ErrGitHubUnavailable
}
