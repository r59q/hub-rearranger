package github

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"

	gh "github.com/google/go-github/v74/github"
	"github.com/r59q/hub-rearranger/services/agents/internal/domain"
	"github.com/r59q/hub-rearranger/services/agents/internal/infrastructure/evidence"
)

const maxArchiveBytes = 65536

func (r *ReadinessReader) readArtifact(ctx context.Context, repo domain.Repository, run *gh.WorkflowRun) (*domain.RuntimeEvidence, error) {
	name := fmt.Sprintf("agent-readiness-v1-%d", run.GetRunAttempt())
	var selected *gh.Artifact
	for page := 1; page <= 5; page++ {
		list, response, err := r.client.Actions.ListWorkflowRunArtifacts(ctx, repo.Owner, repo.Name, run.GetID(), &gh.ListOptions{PerPage: 100, Page: page})
		if err != nil {
			return nil, classify(err, response)
		}

		for _, artifact := range list.Artifacts {
			if artifact.GetName() == name {
				if selected != nil {
					return nil, nil
				}
				selected = artifact
			}
		}

		if response.NextPage == 0 {
			break
		}
		if page == 5 {
			return nil, nil
		}
	}

	if selected == nil || selected.GetExpired() || selected.GetSizeInBytes() > maxArchiveBytes || selected.GetSizeInBytes() <= 0 {
		return nil, nil
	}

	source := selected.WorkflowRun
	if source == nil || source.GetID() != run.GetID() || source.GetHeadSHA() != run.GetHeadSHA() || source.GetHeadBranch() != run.GetHeadBranch() || source.GetRepositoryID() != run.GetRepository().GetID() || source.GetHeadRepositoryID() != run.GetRepository().GetID() {
		return nil, nil
	}

	location, response, err := r.client.Actions.DownloadArtifact(ctx, repo.Owner, repo.Name, selected.GetID(), 0)
	if err != nil {
		if response != nil && (response.StatusCode == http.StatusNotFound || response.StatusCode == http.StatusGone) {
			return nil, nil
		}
		return nil, classify(err, response)
	}
	if location == nil || location.Scheme != "https" || location.User != nil || location.Hostname() == "" {
		return nil, nil
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, location.String(), nil)
	if err != nil {
		return nil, nil
	}
	download, err := r.download.Do(request)
	if err != nil {
		return nil, domain.ErrGitHubUnavailable
	}
	defer download.Body.Close()
	if download.StatusCode == http.StatusNotFound || download.StatusCode == http.StatusGone {
		return nil, nil
	}
	if download.StatusCode != http.StatusOK {
		return nil, domain.ErrGitHubUnavailable
	}

	archive, err := io.ReadAll(io.LimitReader(download.Body, maxArchiveBytes+1))
	if err != nil {
		return nil, domain.ErrGitHubUnavailable
	}
	if len(archive) > maxArchiveBytes {
		return nil, nil
	}

	digest := sha256.Sum256(archive)
	if selected.GetDigest() != "sha256:"+hex.EncodeToString(digest[:]) {
		return nil, nil
	}

	return r.decodeArchive(archive), nil
}

func (r *ReadinessReader) decodeArchive(content []byte) *domain.RuntimeEvidence {
	archive, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil || len(archive.File) != 1 {
		return nil
	}

	file := archive.File[0]
	if file.Name != "readiness.json" || !file.Mode().IsRegular() || file.UncompressedSize64 > evidence.MaxBytes {
		return nil
	}

	body, err := file.Open()
	if err != nil {
		return nil
	}
	defer body.Close()
	record, err := io.ReadAll(io.LimitReader(body, evidence.MaxBytes+1))
	if err != nil {
		return nil
	}

	// A digest constrains bytes, not their shape. Decode with the canonical closed
	// schema before any values reach the domain or a frontend client.
	decoded, err := r.decoder.Decode(record)
	if err != nil {
		return nil
	}
	if strings.TrimSpace(decoded.Repository) == "" {
		return nil
	}
	return decoded
}
