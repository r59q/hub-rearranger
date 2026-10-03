package github

import (
	"context"
	"fmt"
	"strings"

	gh "github.com/google/go-github/v74/github"
	"github.com/r59q/hub-rearranger/services/identity/internal/domain"
)

type AssignmentPublisher struct{ API *gh.Client }

func (p AssignmentPublisher) Assign(ctx context.Context, token string, repo domain.Repository, user domain.User, plan domain.AssignmentPlan, guard func(context.Context) error) (domain.AssignmentResult, error) {
	result := domain.AssignmentResult{}
	if !strings.HasPrefix(token, "ghu_") {
		return result, domain.ErrReconnect
	}
	client := p.API.WithAuthToken(token)
	if err := guard(ctx); err != nil {
		return result, err
	}
	if err := assignmentSource(ctx, client, repo, plan); err != nil {
		return result, err
	}
	existing, last, err := assignmentComments(ctx, client, repo, user, plan)
	if err != nil {
		return result, err
	}
	if existing != nil {
		return assignmentResult(repo, plan.Number, existing), nil
	}
	if last != plan.LastCommentID {
		return result, domain.ErrAssignmentStale
	}
	// Recheck live access and the default head/source immediately before the sole POST.
	if err := guard(ctx); err != nil {
		return result, err
	}
	if err := assignmentSource(ctx, client, repo, plan); err != nil {
		return result, err
	}
	comment, _, createErr := client.Issues.CreateComment(ctx, repo.Owner, repo.Name, int(plan.Number), &gh.IssueComment{Body: gh.Ptr(plan.Command)})
	if createErr == nil && validAssignmentComment(comment, repo, user, plan) {
		return assignmentResult(repo, plan.Number, comment), nil
	}
	// A lost response may have written the comment. Reconcile once using user,
	// exact body, issue URL and original watermark; never send another POST.
	existing, _, readErr := assignmentComments(ctx, client, repo, user, plan)
	if readErr == nil && existing != nil && existing.GetID() > plan.LastCommentID {
		return assignmentResult(repo, plan.Number, existing), nil
	}
	return result, domain.ErrAssignmentUncertain
}

func assignmentSource(ctx context.Context, client *gh.Client, repo domain.Repository, plan domain.AssignmentPlan) error {
	repository, response, err := client.Repositories.Get(ctx, repo.Owner, repo.Name)
	if err != nil {
		return apiError(response, false)
	}
	if repository.GetID() != plan.RepositoryID || !strings.EqualFold(repository.GetFullName(), plan.Repository) || repository.GetArchived() || repository.GetDisabled() || repository.GetDefaultBranch() == "" {
		return domain.ErrForbidden
	}
	branch, response, err := client.Repositories.GetBranch(ctx, repo.Owner, repo.Name, repository.GetDefaultBranch(), 0)
	if err != nil {
		return apiError(response, false)
	}
	if branch.GetCommit().GetSHA() != plan.Revision {
		return domain.ErrAssignmentStale
	}
	issue, response, err := client.Issues.Get(ctx, repo.Owner, repo.Name, int(plan.Number))
	if err != nil {
		return apiError(response, false)
	}
	if issue.IsPullRequest() || issue.GetNumber() != int(plan.Number) || issue.GetState() != "open" || issue.GetLocked() || issue.GetHTMLURL() != fmt.Sprintf("https://github.com/%s/issues/%d", plan.Repository, plan.Number) {
		return domain.ErrAssignmentStale
	}
	return nil
}

func assignmentComments(ctx context.Context, client *gh.Client, repo domain.Repository, user domain.User, plan domain.AssignmentPlan) (*gh.IssueComment, int64, error) {
	var found *gh.IssueComment
	var last int64
	for page := 1; page <= 20; page++ {
		comments, response, err := client.Issues.ListComments(ctx, repo.Owner, repo.Name, int(plan.Number), &gh.IssueListCommentsOptions{ListOptions: gh.ListOptions{PerPage: 100, Page: page}})
		if err != nil {
			return nil, 0, apiError(response, false)
		}
		for _, comment := range comments {
			if comment.GetID() <= 0 {
				return nil, 0, domain.ErrAssignmentStale
			}
			if comment.GetID() > last {
				last = comment.GetID()
			}
			if comment.GetBody() != plan.Command {
				continue
			}
			if found != nil || !validAssignmentComment(comment, repo, user, plan) {
				return nil, 0, domain.ErrAssignmentStale
			}
			found = comment
		}
		if response.NextPage == 0 {
			return found, last, nil
		}
	}
	return nil, 0, domain.ErrAssignmentStale
}

func validAssignmentComment(comment *gh.IssueComment, repo domain.Repository, user domain.User, plan domain.AssignmentPlan) bool {
	return comment != nil && comment.GetID() > 0 && comment.GetBody() == plan.Command && comment.GetUser().GetID() == user.ID && comment.GetUser().GetType() == "User" && strings.EqualFold(comment.GetUser().GetLogin(), user.Login) && comment.GetHTMLURL() == fmt.Sprintf("https://github.com/%s/%s/issues/%d#issuecomment-%d", repo.Owner, repo.Name, plan.Number, comment.GetID()) && !comment.GetCreatedAt().Time.IsZero() && comment.GetCreatedAt().Time.Equal(comment.GetUpdatedAt().Time)
}

func assignmentResult(repo domain.Repository, number int64, comment *gh.IssueComment) domain.AssignmentResult {
	return domain.AssignmentResult{Repository: repo.Owner + "/" + repo.Name, Number: number, CommentID: comment.GetID(), CommentURL: comment.GetHTMLURL()}
}
