package github

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"

	gh "github.com/google/go-github/v74/github"
	"github.com/r59q/hub-rearranger/services/agents/internal/domain"
	"github.com/r59q/hub-rearranger/services/agents/internal/infrastructure/evidence"
)

var assignmentCommand = regexp.MustCompile(`^/agent assign codex-thorough@([0-9a-f]{40}) authority=branch-draft-pr$`)

type AssignmentReader struct {
	client   *gh.Client
	download *http.Client
	decoder  *evidence.ResultDecoder
}

func NewAssignmentReader(client *gh.Client, decoder *evidence.ResultDecoder) *AssignmentReader {
	return &AssignmentReader{client: client, download: assignmentDownloadClient(), decoder: decoder}
}

func (r *AssignmentReader) ReadAssignment(ctx context.Context, repo domain.Repository, number int64) (domain.IssueAssignment, error) {
	result := domain.IssueAssignment{Repository: repo.FullName(), Number: number, Requests: []domain.AssignmentRequest{}}
	repository, response, err := r.client.Repositories.Get(ctx, repo.Owner, repo.Name)
	if err != nil {
		return result, classify(err, response)
	}
	if repository.GetID() <= 0 || !strings.EqualFold(repository.GetFullName(), repo.FullName()) || repository.GetDefaultBranch() == "" {
		return result, domain.ErrRepositoryUnavailable
	}
	result.RepositoryID = repository.GetID()
	issue, response, err := r.client.Issues.Get(ctx, repo.Owner, repo.Name, int(number))
	if err != nil {
		return result, classify(err, response)
	}
	result.URL = fmt.Sprintf("https://github.com/%s/issues/%d", repo.FullName(), number)
	if issue.IsPullRequest() || issue.GetNumber() != int(number) || issue.GetHTMLURL() != result.URL || (issue.GetState() != "open" && issue.GetState() != "closed") {
		return result, domain.ErrRepositoryUnavailable
	}
	result.IssueLocked = issue.GetLocked()
	result.Title, result.Body, result.IssueState = truncate(issue.GetTitle(), 512), truncate(issue.GetBody(), 4000), issue.GetState()
	for page := 1; page <= 20; page++ {
		comments, response, err := r.client.Issues.ListComments(ctx, repo.Owner, repo.Name, int(number), &gh.IssueListCommentsOptions{ListOptions: gh.ListOptions{PerPage: 100, Page: page}})
		if err != nil {
			return result, classify(err, response)
		}
		for _, comment := range comments {
			if comment.GetID() <= 0 {
				return result, domain.ErrGitHubUnavailable
			}
			if comment.GetID() > result.LastCommentID {
				result.LastCommentID = comment.GetID()
			}
			command := assignmentCommand.FindStringSubmatch(comment.GetBody())
			if command == nil || comment.GetUser().GetType() != "User" || comment.GetUser().GetID() <= 0 || comment.GetCreatedAt().Time.IsZero() {
				continue
			}
			url := fmt.Sprintf("%s#issuecomment-%d", result.URL, comment.GetID())
			if comment.GetHTMLURL() != url {
				return result, domain.ErrGitHubUnavailable
			}
			result.Requests = append(result.Requests, domain.AssignmentRequest{CommentID: comment.GetID(), IssueNumber: number, URL: url, RequesterID: comment.GetUser().GetID(), Requester: comment.GetUser().GetLogin(), ProfileRevision: command[1], CreatedAt: comment.GetCreatedAt().Time, Edited: !comment.GetCreatedAt().Time.Equal(comment.GetUpdatedAt().Time), State: "requested", Summary: "Request exists on GitHub. Awaiting workflow evidence; it may be delayed or not enabled."})
		}
		if response.NextPage == 0 {
			break
		}
		if page == 20 {
			return result, domain.ErrGitHubUnavailable
		}
	}
	sort.Slice(result.Requests, func(i, j int) bool { return result.Requests[i].CommentID > result.Requests[j].CommentID })
	if len(result.Requests) > 10 {
		result.Requests = result.Requests[:10]
	}
	if len(result.Requests) == 0 {
		return result, nil
	}
	runs, err := r.assignmentRuns(ctx, repo, result.RepositoryID, result.Requests)
	if err != nil {
		for i := range result.Requests {
			result.Requests[i].State = "unavailable"
			result.Requests[i].Summary = "Workflow evidence is unavailable. Open the request on GitHub and refresh later."
		}
		return result, nil
	}
	for i := range result.Requests {
		request := &result.Requests[i]
		run := runs[request.CommentID]
		if run == nil {
			continue
		}
		request.Run = &domain.AssignmentRun{ID: run.GetID(), Attempt: int64(run.GetRunAttempt()), URL: fmt.Sprintf("https://github.com/%s/actions/runs/%d", repo.FullName(), run.GetID()), Status: run.GetStatus(), Conclusion: run.GetConclusion()}
		request.State, request.Summary = runState(run)
		if run.GetStatus() == "completed" {
			state, summary, err := r.assignmentJobState(ctx, repo, run)
			if err != nil {
				request.State = "unavailable"
				request.Summary = "Workflow job evidence is unavailable. Inspect GitHub before retrying."
			} else if state != "" {
				request.State, request.Summary = state, summary
			}
		}
		proposal, err := r.assignmentProposal(ctx, repo, repository, request, run)
		if err != nil {
			request.State = "unavailable"
			request.Summary = "Proposal evidence is incomplete or changed. Inspect the original workflow and GitHub objects before retrying."
			continue
		}
		request.Proposal = proposal
		if proposal != nil {
			request.State = "proposal"
			request.Summary = "GitHub has a linked patch proposal. Review its current PR state and checks; human review is required."
		}
	}
	return result, nil
}

func truncate(value string, limit int) string {
	text := []rune(value)
	if len(text) > limit {
		return string(text[:limit])
	}
	return value
}

func (r *AssignmentReader) assignmentRuns(ctx context.Context, repo domain.Repository, repoID int64, requests []domain.AssignmentRequest) (map[int64]*gh.WorkflowRun, error) {
	selected := map[int64]*gh.WorkflowRun{}
	oldest := requests[len(requests)-1].CreatedAt.Add(-time.Second).UTC().Format(time.RFC3339)
	for page := 1; page <= 10; page++ {
		list, response, err := r.client.Actions.ListWorkflowRunsByFileName(ctx, repo.Owner, repo.Name, "agent-assignment.yml", &gh.ListWorkflowRunsOptions{Event: "issue_comment", Created: ">=" + oldest, ListOptions: gh.ListOptions{PerPage: 100, Page: page}})
		if err != nil {
			return nil, classify(err, response)
		}
		if list.GetTotalCount() > 1000 {
			return nil, domain.ErrGitHubUnavailable
		}
		for _, run := range list.WorkflowRuns {
			for _, request := range requests {
				if validAssignmentRun(run, repoID, request) && (selected[request.CommentID] == nil || run.GetID() < selected[request.CommentID].GetID()) {
					selected[request.CommentID] = run
				}
			}
		}
		if response.NextPage == 0 {
			return selected, nil
		}
	}
	return nil, domain.ErrGitHubUnavailable
}

func validAssignmentRun(run *gh.WorkflowRun, repoID int64, request domain.AssignmentRequest) bool {
	return run.GetID() > 0 && run.GetRunAttempt() > 0 && run.GetRepository().GetID() == repoID && run.GetHeadRepository().GetID() == repoID && run.GetEvent() == "issue_comment" && strings.SplitN(run.GetPath(), "@", 2)[0] == ".github/workflows/agent-assignment.yml" && run.GetDisplayTitle() == fmt.Sprintf("Agent assignment %d:%d", repoID, request.CommentID) && run.GetActor().GetID() == request.RequesterID && strings.EqualFold(run.GetActor().GetLogin(), request.Requester) && commitSHA.MatchString(run.GetHeadSHA()) && !run.GetCreatedAt().Time.IsZero() && !run.GetCreatedAt().Before(request.CreatedAt.Add(-time.Second))
}

func runState(run *gh.WorkflowRun) (string, string) {
	if run.GetStatus() == "completed" {
		switch run.GetConclusion() {
		case "cancelled":
			return "cancelled", "Workflow cancelled. Inspect the original attempt before retrying."
		case "failure", "timed_out", "action_required", "startup_failure":
			return "failed", "Workflow failed. Open GitHub for job diagnostics and recovery guidance."
		default:
			return "completed", "Workflow completed. Completion alone does not prove execution, passing validation or publication. Open its job summary for blocked or skipped phases."
		}
	}
	if run.GetStatus() == "in_progress" {
		return "running", "GitHub reports the assignment workflow in progress."
	}
	return "queued", "GitHub reports the assignment workflow waiting."
}
