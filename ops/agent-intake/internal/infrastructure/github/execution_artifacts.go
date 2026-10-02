package github

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/url"
	"time"

	gh "github.com/google/go-github/v74/github"
	"github.com/r59q/hub-rearranger/ops/agent-intake/internal/domain"
)

const maxExecutionArchive = 12 * 1024 * 1024

// ExecutionArtifact verifies GitHub's immutable archive digest and origin before
// decoding any claims. The separate storage request never carries a GitHub token.
func (c *Client) ExecutionArtifact(ctx context.Context, repo domain.Repository, run *gh.WorkflowRun, name string) (map[string][]byte, int64, error) {
	var selected *gh.Artifact
	for page := 1; page <= 5; page++ {
		list, response, err := c.api.Actions.ListWorkflowRunArtifacts(ctx, repo.Owner, repo.Name, run.GetID(), &gh.ListOptions{PerPage: 100, Page: page})
		if err != nil {
			return nil, 0, domain.Unavailable
		}
		for _, artifact := range list.Artifacts {
			if artifact.GetName() == name {
				if selected != nil {
					return nil, 0, domain.HistoryUnavailable
				}
				selected = artifact
			}
		}
		if response.NextPage == 0 {
			break
		}
		if page == 5 {
			return nil, 0, domain.HistoryUnavailable
		}
	}
	if selected == nil || selected.GetID() <= 0 || selected.GetExpired() || selected.GetSizeInBytes() <= 0 || selected.GetSizeInBytes() > maxExecutionArchive {
		return nil, 0, domain.HistoryUnavailable
	}
	origin := selected.WorkflowRun
	if origin == nil || origin.GetID() != run.GetID() || origin.GetRepositoryID() != repo.ID || origin.GetHeadRepositoryID() != repo.ID || origin.GetHeadSHA() != run.GetHeadSHA() || origin.GetHeadBranch() != repo.DefaultBranch {
		return nil, 0, domain.HistoryUnavailable
	}
	location, _, err := c.api.Actions.DownloadArtifact(ctx, repo.Owner, repo.Name, selected.GetID(), 0)
	if err != nil {
		return nil, 0, domain.Unavailable
	}
	archive, err := c.downloadExecution(ctx, location, maxExecutionArchive)
	if err != nil {
		return nil, 0, err
	}
	digest := sha256.Sum256(archive)
	if selected.GetDigest() != "sha256:"+hex.EncodeToString(digest[:]) {
		return nil, 0, domain.HistoryUnavailable
	}
	files, err := decodeExecutionArchive(archive)
	return files, selected.GetID(), err
}

func (c *Client) downloadExecution(ctx context.Context, location *url.URL, limit int64) ([]byte, error) {
	if location == nil || location.Scheme != "https" || location.Hostname() == "" || location.User != nil {
		return nil, domain.HistoryUnavailable
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, location.String(), nil)
	if err != nil {
		return nil, domain.HistoryUnavailable
	}
	client := &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	if c.download != nil {
		client.Transport = c.download.Transport
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, domain.Unavailable
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return nil, domain.Unavailable
	}
	content, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil || int64(len(content)) > limit {
		return nil, domain.HistoryUnavailable
	}
	return content, nil
}

func decodeExecutionArchive(content []byte) (map[string][]byte, error) {
	archive, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil || len(archive.File) < 1 || len(archive.File) > 3 {
		return nil, domain.HistoryUnavailable
	}
	files := map[string][]byte{}
	total := 0
	for _, file := range archive.File {
		if !file.Mode().IsRegular() || file.UncompressedSize64 > maxExecutionArchive || files[file.Name] != nil {
			return nil, domain.HistoryUnavailable
		}
		switch file.Name {
		case "invocation.json", "readiness.json", "result.json", "proposal.patch", "summary.json":
		default:
			return nil, domain.HistoryUnavailable
		}
		body, err := file.Open()
		if err != nil {
			return nil, domain.HistoryUnavailable
		}
		data, err := io.ReadAll(io.LimitReader(body, maxExecutionArchive+1))
		_ = body.Close()
		total += len(data)
		if err != nil || total > maxExecutionArchive {
			return nil, domain.HistoryUnavailable
		}
		files[file.Name] = data
	}
	return files, nil
}
