// Package github implements intake's GitHub boundary with go-github.
package github

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	gh "github.com/google/go-github/v74/github"
	"github.com/r59q/hub-rearranger/ops/agent-intake/internal/domain"
)

type Client struct {
	api      *gh.Client
	download *http.Client
}

func New(client *gh.Client) *Client { return &Client{api: client} }

func safeError(err error) error {
	var response *gh.ErrorResponse
	if errors.As(err, &response) && response.Response != nil && response.Response.StatusCode == http.StatusNotFound {
		return domain.SourceChanged
	}
	return domain.Unavailable
}

func (c *Client) Repository(ctx context.Context, event domain.Event) (domain.Repository, error) {
	repo, _, err := c.api.Repositories.Get(ctx, event.Repository.Owner, event.Repository.Name)
	if err != nil {
		return domain.Repository{}, safeError(err)
	}

	return domain.Repository{ID: repo.GetID(), Owner: repo.GetOwner().GetLogin(), Name: repo.GetName(),
		DefaultBranch: repo.GetDefaultBranch(), Fork: repo.GetFork(), Archived: repo.GetArchived() || repo.GetDisabled(), Private: repo.GetPrivate()}, nil
}

func (c *Client) Source(ctx context.Context, repo domain.Repository, event domain.Event) (domain.Source, error) {
	comment, _, err := c.api.Issues.GetComment(ctx, repo.Owner, repo.Name, event.CommentID)
	if err != nil {
		return domain.Source{}, safeError(err)
	}

	issue, _, err := c.api.Issues.Get(ctx, repo.Owner, repo.Name, event.IssueNumber)
	if err != nil {
		return domain.Source{}, safeError(err)
	}

	expected := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues/%d", repo.Owner, repo.Name, event.IssueNumber)
	if !strings.EqualFold(comment.GetIssueURL(), expected) || !strings.EqualFold(issue.GetURL(), expected) || comment.GetUser().GetType() != "User" {
		return domain.Source{}, domain.SourceChanged
	}

	return domain.Source{IssueNumber: issue.GetNumber(), CommentID: comment.GetID(), Body: comment.GetBody(),
		Requester: domain.User{ID: comment.GetUser().GetID(), Login: comment.GetUser().GetLogin()},
		Created:   comment.GetCreatedAt().Time, Updated: comment.GetUpdatedAt().Time,
		Open: issue.GetState() == "open", PullRequest: issue.IsPullRequest()}, nil
}

func (c *Client) Role(ctx context.Context, repo domain.Repository, login string) (domain.User, string, error) {
	permission, _, err := c.api.Repositories.GetPermissionLevel(ctx, repo.Owner, repo.Name, login)
	if err != nil {
		return domain.User{}, "", safeError(err)
	}

	user := permission.GetUser()
	return domain.User{ID: user.GetID(), Login: user.GetLogin()}, permission.GetRoleName(), nil
}

func (c *Client) Head(ctx context.Context, repo domain.Repository) (string, error) {
	ref, _, err := c.api.Git.GetRef(ctx, repo.Owner, repo.Name, "heads/"+repo.DefaultBranch)
	if err != nil {
		return "", safeError(err)
	}
	return ref.GetObject().GetSHA(), nil
}

func (c *Client) Ancestor(ctx context.Context, repo domain.Repository, base, head string) (bool, error) {
	if base == head {
		return true, nil
	}

	comparison, _, err := c.api.Repositories.CompareCommits(ctx, repo.Owner, repo.Name, base, head, &gh.ListOptions{PerPage: 1})
	if err != nil {
		return false, safeError(err)
	}

	return comparison.GetBaseCommit().GetSHA() == base && comparison.GetMergeBaseCommit().GetSHA() == base &&
		(comparison.GetStatus() == "ahead" || comparison.GetStatus() == "identical"), nil
}

func (c *Client) Catalog(ctx context.Context, repo domain.Repository, sha string) ([]byte, error) {
	file, _, _, err := c.api.Repositories.GetContents(ctx, repo.Owner, repo.Name, domain.CatalogPath, &gh.RepositoryContentGetOptions{Ref: sha})
	if err != nil {
		return nil, safeError(err)
	}
	if file == nil || file.GetType() != "file" || file.GetSize() > 65_536 || file.GetSize() <= 0 {
		return nil, domain.ProfileInvalid
	}

	content, err := file.GetContent()
	if err != nil || len(content) > 65_536 {
		return nil, domain.ProfileInvalid
	}
	return []byte(content), nil
}
