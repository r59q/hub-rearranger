// Package github projects current repository configuration through go-github.
package github

import (
	"context"
	"errors"
	"net/http"
	"regexp"

	gh "github.com/google/go-github/v74/github"

	"github.com/r59q/hub-rearranger/services/agents/internal/domain"
	"github.com/r59q/hub-rearranger/services/agents/internal/infrastructure/profiles"
)

var commitSHA = regexp.MustCompile(`^[0-9a-f]{40}$`)

type ProfileReader struct{ client *gh.Client }

func NewProfileReader(client *gh.Client) *ProfileReader { return &ProfileReader{client: client} }

func (r *ProfileReader) ReadCatalog(ctx context.Context, repository domain.Repository) (domain.CatalogFile, error) {
	repo, response, err := r.client.Repositories.Get(ctx, repository.Owner, repository.Name)
	if err != nil {
		return domain.CatalogFile{}, classify(err, response)
	}
	file := domain.CatalogFile{DefaultBranch: repo.GetDefaultBranch()}
	if file.DefaultBranch == "" {
		return file, domain.ErrRepositoryUnavailable
	}
	branch, response, err := r.client.Repositories.GetBranch(ctx, repository.Owner, repository.Name, file.DefaultBranch, 0)
	if err != nil {
		return file, classify(err, response)
	}
	file.Revision = branch.GetCommit().GetSHA()
	if !commitSHA.MatchString(file.Revision) {
		return file, domain.ErrGitHubUnavailable
	}
	content, directory, response, err := r.client.Repositories.GetContents(ctx, repository.Owner, repository.Name,
		domain.ProfileCatalogPath, &gh.RepositoryContentGetOptions{Ref: file.Revision})
	if err != nil {
		if response != nil && response.StatusCode == http.StatusNotFound {
			file.Problem = &domain.Diagnostic{Code: "MISSING_FILE", Path: "/", Message: "Add .github/agent-profiles.yml to the default branch."}
			return file, nil
		}
		return file, classify(err, response)
	}
	if content == nil || directory != nil || content.GetType() != "file" || content.GetTarget() != "" || content.GetSubmoduleGitURL() != "" {
		file.Problem = &domain.Diagnostic{Code: "INVALID_FILE", Path: "/", Message: "Replace .github/agent-profiles.yml with a regular YAML file."}
		return file, nil
	}
	if content.GetSize() > profiles.MaxBytes {
		file.Problem = &domain.Diagnostic{Code: "LIMIT_EXCEEDED", Path: "/", Message: "Reduce the catalog to at most 65536 bytes."}
		return file, nil
	}
	decoded, err := content.GetContent()
	if err != nil || content.GetEncoding() != "base64" || content.Content == nil {
		return file, domain.ErrGitHubUnavailable
	}
	file.Content = []byte(decoded)
	return file, nil
}

func classify(err error, upstream *gh.Response) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	var rate *gh.RateLimitError
	var abuse *gh.AbuseRateLimitError
	if errors.As(err, &rate) || errors.As(err, &abuse) {
		return domain.ErrRateLimited
	}
	var response *gh.ErrorResponse
	status := 0
	if errors.As(err, &response) && response.Response != nil {
		status = response.Response.StatusCode
	}
	if upstream != nil {
		status = upstream.StatusCode
		if status == http.StatusForbidden && (upstream.Header.Get("X-RateLimit-Remaining") == "0" || upstream.Header.Get("Retry-After") != "") {
			return domain.ErrRateLimited
		}
	}
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return domain.ErrAccessDenied
	case http.StatusNotFound, http.StatusConflict:
		return domain.ErrRepositoryUnavailable
	case http.StatusTooManyRequests:
		return domain.ErrRateLimited
	}
	return domain.ErrGitHubUnavailable
}
