package github

import (
	"context"
	"errors"
	"net/http"

	gh "github.com/google/go-github/v74/github"
	"github.com/r59q/hub-rearranger/ops/agent-intake/internal/domain"
)

func notFound(err error) bool {
	var response *gh.ErrorResponse
	return errors.As(err, &response) && response.Response != nil && response.Response.StatusCode == http.StatusNotFound
}

func (c *Client) branch(ctx context.Context, input domain.Invocation, head string) (bool, error) {
	repo := input.Repository
	name := "heads/" + domain.PublicationBranch(input)
	ref, _, err := c.api.Git.GetRef(ctx, repo.Owner, repo.Name, name)
	if notFound(err) {
		return false, nil
	}
	if err != nil {
		return false, domain.Unavailable
	}
	if ref.GetRef() != "refs/"+name || ref.GetObject().GetType() != "commit" || ref.GetObject().GetSHA() != head {
		return false, domain.StaleHead
	}
	return true, nil
}

// EnsureBranch only creates a missing deterministic ref. It never updates,
// deletes, force-pushes, or overwrites an existing ref, including after lost POST.
func (c *Client) EnsureBranch(ctx context.Context, input domain.Invocation, head string) error {
	exists, err := c.branch(ctx, input, head)
	if err != nil || exists {
		return err
	}
	repo := input.Repository
	_, _, _ = c.api.Git.CreateRef(ctx, repo.Owner, repo.Name, &gh.Reference{Ref: gh.Ptr("refs/heads/" + domain.PublicationBranch(input)), Object: &gh.GitObject{SHA: gh.Ptr(head)}})
	exists, err = c.branch(ctx, input, head)
	if err != nil {
		return err
	}
	if !exists {
		return domain.HistoryUnavailable
	}
	return nil
}
