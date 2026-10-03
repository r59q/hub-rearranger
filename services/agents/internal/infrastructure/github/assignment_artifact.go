package github

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	gh "github.com/google/go-github/v74/github"
	"github.com/r59q/hub-rearranger/services/agents/internal/domain"
	"github.com/r59q/hub-rearranger/services/agents/internal/infrastructure/profiles"
)

const maxProposalArchive = 12 * 1024 * 1024

func digest(content []byte) string {
	value := sha256.Sum256(content)
	return hex.EncodeToString(value[:])
}

func (r *AssignmentReader) verifyProposalArtifact(ctx context.Context, repo domain.Repository, repository *gh.Repository, request *domain.AssignmentRequest, run *gh.WorkflowRun, publication publicationMetadata) (string, error) {
	if err := r.verifyProposalJobs(ctx, repo, run); err != nil {
		return "", err
	}
	files, id, err := r.assignmentArtifact(ctx, repo, run, fmt.Sprintf("agent-patch-v1-%d", publication.Attempt))
	if err != nil || id != publication.ArtifactID || len(files) != 3 {
		return "", domain.ErrGitHubUnavailable
	}
	result, err := r.decoder.Decode(files["result.json"])
	if err != nil || result.Outcome != "ready" || result.Proposal == nil || result.AssignmentID != publication.AssignmentID || result.ProfileRevision != request.ProfileRevision || result.RunID != run.GetID() || result.Attempt != publication.Attempt || result.BaseSHA != publication.BaseSHA || result.Proposal.PatchSHA256 != publication.PatchSHA256 || digest(files["proposal.patch"]) != publication.PatchSHA256 || digest(files["summary.json"]) != result.Proposal.SummarySHA256 {
		return "", domain.ErrGitHubUnavailable
	}
	invocation, _, err := r.assignmentArtifact(ctx, repo, run, fmt.Sprintf("agent-invocation-v1-%d", publication.Attempt))
	if err != nil || len(invocation) != 1 || len(invocation["invocation.json"]) > 65536 {
		return "", domain.ErrGitHubUnavailable
	}
	var source struct {
		ContractVersion int    `json:"contract_version"`
		Operation       string `json:"operation"`
		ProfileID       string `json:"profile_id"`
		Authority       string `json:"authority"`
		SourceURL       string `json:"source_url"`
		RequestURL      string `json:"request_url"`
		RunURL          string `json:"run_url"`

		AssignmentID string `json:"assignment_id"`
		Repository   struct {
			ID    int64  `json:"id"`
			Owner string `json:"owner"`
			Name  string `json:"name"`
		} `json:"repository"`
		IssueNumber      int   `json:"issue_number"`
		RequestCommentID int64 `json:"request_comment_id"`
		Requester        struct {
			ID    int64  `json:"id"`
			Login string `json:"login"`
		} `json:"requester"`
		ProfileRevision string `json:"profile_revision"`
		BaseSHA         string `json:"base_sha"`
		RunID           int64  `json:"run_id"`
		Attempt         int    `json:"run_attempt"`
	}
	_, valid := profiles.ParseDocument(invocation["invocation.json"])
	if !valid || json.Unmarshal(invocation["invocation.json"], &source) != nil || source.ContractVersion != 1 || source.Operation != "assignment" || source.ProfileID != "codex-thorough" || source.Authority != "branch-draft-pr" || source.SourceURL != fmt.Sprintf("https://github.com/%s/issues/%d", repo.FullName(), requestIssue(request)) || source.RequestURL != request.URL || source.RunURL != fmt.Sprintf("https://github.com/%s/actions/runs/%d/attempts/%d", repo.FullName(), run.GetID(), run.GetRunAttempt()) || source.AssignmentID != publication.AssignmentID || source.Repository.ID != repository.GetID() || source.Repository.Owner != repo.Owner || source.Repository.Name != repo.Name || source.IssueNumber != requestIssue(request) || source.RequestCommentID != request.CommentID || source.Requester.ID != request.RequesterID || source.Requester.Login != request.Requester || source.ProfileRevision != request.ProfileRevision || source.BaseSHA != publication.BaseSHA || source.RunID != run.GetID() || source.Attempt != publication.Attempt {
		return "", domain.ErrGitHubUnavailable
	}
	return result.Validation[0].Outcome, nil
}

// The separate storage client never carries GitHub authentication or follows redirects.
func (r *AssignmentReader) assignmentArtifact(ctx context.Context, repo domain.Repository, run *gh.WorkflowRun, name string) (map[string][]byte, int64, error) {
	var selected *gh.Artifact
	for page := 1; page <= 5; page++ {
		list, response, err := r.client.Actions.ListWorkflowRunArtifacts(ctx, repo.Owner, repo.Name, run.GetID(), &gh.ListOptions{PerPage: 100, Page: page})
		if err != nil {
			return nil, 0, classify(err, response)
		}
		for _, artifact := range list.Artifacts {
			if artifact.GetName() == name {
				if selected != nil {
					return nil, 0, domain.ErrGitHubUnavailable
				}
				selected = artifact
			}
		}
		if response.NextPage == 0 {
			break
		}
		if page == 5 {
			return nil, 0, domain.ErrGitHubUnavailable
		}
	}
	if selected == nil || selected.GetID() <= 0 || selected.GetExpired() || selected.GetSizeInBytes() <= 0 || selected.GetSizeInBytes() > maxProposalArchive {
		return nil, 0, domain.ErrGitHubUnavailable
	}
	origin := selected.WorkflowRun
	if origin == nil || origin.GetID() != run.GetID() || origin.GetRepositoryID() != run.GetRepository().GetID() || origin.GetHeadRepositoryID() != run.GetRepository().GetID() || origin.GetHeadSHA() != run.GetHeadSHA() || origin.GetHeadBranch() != run.GetHeadBranch() {
		return nil, 0, domain.ErrGitHubUnavailable
	}
	location, response, err := r.client.Actions.DownloadArtifact(ctx, repo.Owner, repo.Name, selected.GetID(), 0)
	if err != nil {
		return nil, 0, classify(err, response)
	}
	if location == nil || location.Scheme != "https" || location.User != nil || location.Hostname() == "" {
		return nil, 0, domain.ErrGitHubUnavailable
	}
	storageRequest, err := http.NewRequestWithContext(ctx, "GET", location.String(), nil)
	if err != nil {
		return nil, 0, domain.ErrGitHubUnavailable
	}
	storage, err := r.download.Do(storageRequest)
	if err != nil {
		return nil, 0, domain.ErrGitHubUnavailable
	}
	defer storage.Body.Close()
	if storage.StatusCode != 200 {
		return nil, 0, domain.ErrGitHubUnavailable
	}
	content, err := io.ReadAll(io.LimitReader(storage.Body, maxProposalArchive+1))
	if err != nil || len(content) > maxProposalArchive || selected.GetDigest() != "sha256:"+digest(content) {
		return nil, 0, domain.ErrGitHubUnavailable
	}
	files, err := proposalArchive(content)
	return files, selected.GetID(), err
}

func proposalArchive(content []byte) (map[string][]byte, error) {
	archive, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil || len(archive.File) < 1 || len(archive.File) > 3 {
		return nil, domain.ErrGitHubUnavailable
	}
	files := map[string][]byte{}
	total := 0
	for _, file := range archive.File {
		if !file.Mode().IsRegular() || file.UncompressedSize64 > maxProposalArchive || files[file.Name] != nil {
			return nil, domain.ErrGitHubUnavailable
		}
		switch file.Name {
		case "invocation.json", "result.json", "proposal.patch", "summary.json":
		default:
			return nil, domain.ErrGitHubUnavailable
		}
		body, err := file.Open()
		if err != nil {
			return nil, domain.ErrGitHubUnavailable
		}
		data, err := io.ReadAll(io.LimitReader(body, maxProposalArchive+1))
		_ = body.Close()
		total += len(data)
		if err != nil || total > maxProposalArchive {
			return nil, domain.ErrGitHubUnavailable
		}
		files[file.Name] = data
	}
	return files, nil
}

func assignmentDownloadClient() *http.Client {
	return &http.Client{Timeout: 8 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
}
