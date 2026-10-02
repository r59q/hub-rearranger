package github

import (
	"context"
	"encoding/json"
	"slices"

	gh "github.com/google/go-github/v74/github"
	"github.com/r59q/hub-rearranger/ops/agent-intake/internal/domain"
)

// ExecutionContext collects only this assignment's current permitted issue
// context. Transport types and GitHub tokens never cross into the workload.
func (c *Client) ExecutionContext(ctx context.Context, input domain.Invocation) ([]byte, error) {
	var policy struct {
		Context struct {
			Sources []string `json:"sources"`
		} `json:"context"`
	}
	if json.Unmarshal(input.Profile, &policy) != nil || !slices.Contains(policy.Context.Sources, "issue") {
		return nil, domain.ProfileInvalid
	}
	repo := input.Repository
	issue, _, err := c.api.Issues.Get(ctx, repo.Owner, repo.Name, input.IssueNumber)
	if err != nil || issue.GetState() != "open" || issue.IsPullRequest() || len(issue.GetTitle()) > 1024 || len(issue.GetBody()) > 65536 {
		return nil, domain.SourceChanged
	}
	result := map[string]any{"issue": map[string]string{"title": issue.GetTitle(), "body": issue.GetBody()}}
	if slices.Contains(policy.Context.Sources, "issue_comments") {
		var comments []map[string]string
		for page := 1; page <= 5; page++ {
			values, response, err := c.api.Issues.ListComments(ctx, repo.Owner, repo.Name, input.IssueNumber, &gh.IssueListCommentsOptions{ListOptions: gh.ListOptions{Page: page, PerPage: 100}})
			if err != nil {
				return nil, domain.Unavailable
			}
			for _, comment := range values {
				if len(comment.GetBody()) > 65536 {
					return nil, domain.Invalid
				}
				comments = append(comments, map[string]string{"author": comment.GetUser().GetLogin(), "body": comment.GetBody()})
			}
			if response.NextPage == 0 {
				break
			}
			if page == 5 {
				return nil, domain.HistoryUnavailable
			}
		}
		result["comments"] = comments
	}
	content, err := json.Marshal(result)
	if err != nil || len(content) > 262144 {
		return nil, domain.Invalid
	}
	return content, nil
}

func (c *Client) ExecutionSource(ctx context.Context, input domain.Invocation) ([]byte, error) {
	location, _, err := c.api.Repositories.GetArchiveLink(ctx, input.Repository.Owner, input.Repository.Name, gh.Tarball, &gh.RepositoryContentGetOptions{Ref: input.BaseSHA}, 0)
	if err != nil {
		return nil, domain.Unavailable
	}
	return c.downloadExecution(ctx, location, 32*1024*1024)
}
