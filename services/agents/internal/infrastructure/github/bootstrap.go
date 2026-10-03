package github

import (
	"context"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/r59q/hub-rearranger/services/agents/internal/domain"
)

func (r *ProfileReader) ReadBootstrap(ctx context.Context, repository domain.Repository, templates map[string]string) (domain.BootstrapSnapshot, error) {
	repo, response, err := r.client.Repositories.Get(ctx, repository.Owner, repository.Name)
	if err != nil {
		return domain.BootstrapSnapshot{}, classify(err, response)
	}
	if repo.GetArchived() || repo.GetDisabled() || !strings.EqualFold(repo.GetFullName(), repository.FullName()) || repo.GetDefaultBranch() == "" {
		return domain.BootstrapSnapshot{}, domain.ErrRepositoryUnavailable
	}
	branch, response, err := r.client.Repositories.GetBranch(ctx, repository.Owner, repository.Name, repo.GetDefaultBranch(), 0)
	if err != nil {
		return domain.BootstrapSnapshot{}, classify(err, response)
	}
	revision := branch.GetCommit().GetSHA()
	if !commitSHA.MatchString(revision) {
		return domain.BootstrapSnapshot{}, domain.ErrGitHubUnavailable
	}
	commit, response, err := r.client.Git.GetCommit(ctx, repository.Owner, repository.Name, revision)
	if err != nil {
		return domain.BootstrapSnapshot{}, classify(err, response)
	}
	if commit.GetSHA() != revision || !commitSHA.MatchString(commit.GetTree().GetSHA()) {
		return domain.BootstrapSnapshot{}, domain.ErrGitHubUnavailable
	}
	tree, response, err := r.client.Git.GetTree(ctx, repository.Owner, repository.Name, commit.GetTree().GetSHA(), true)
	if err != nil {
		return domain.BootstrapSnapshot{}, classify(err, response)
	}
	if tree.GetTruncated() || tree.GetSHA() != commit.GetTree().GetSHA() {
		return domain.BootstrapSnapshot{}, domain.ErrGitHubUnavailable
	}
	snapshot := domain.BootstrapSnapshot{DefaultBranch: repo.GetDefaultBranch(), Revision: revision, Private: repo.GetPrivate(), Files: map[string]string{}, Blobs: map[string]string{}}
	seen := map[string]bool{}
	for _, entry := range tree.Entries {
		path := entry.GetPath()
		if seen[path] {
			return domain.BootstrapSnapshot{}, domain.ErrGitHubUnavailable
		}
		seen[path] = true
		proposed, wanted := templates[path]
		if !wanted {
			// Reject a symlink/submodule/file used as a parent of an installed path.
			if entry.GetType() != "tree" {
				for candidate := range templates {
					if strings.HasPrefix(candidate, path+"/") {
						return domain.BootstrapSnapshot{}, domain.ErrRepositoryUnavailable
					}
				}
			}
			continue
		}
		if entry.GetType() != "blob" || entry.GetMode() != "100644" || !commitSHA.MatchString(entry.GetSHA()) || entry.GetSize() > domain.BootstrapMaxBytes {
			return domain.BootstrapSnapshot{}, domain.ErrRepositoryUnavailable
		}
		snapshot.Blobs[path] = entry.GetSHA()
		if fmt.Sprintf("%x", sha1.Sum([]byte(fmt.Sprintf("blob %d\x00%s", len(proposed), proposed)))) == entry.GetSHA() {
			snapshot.Files[path] = proposed
			continue
		}
		blob, response, err := r.client.Git.GetBlob(ctx, repository.Owner, repository.Name, entry.GetSHA())
		if err != nil {
			return domain.BootstrapSnapshot{}, classify(err, response)
		}
		content, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(blob.GetContent(), "\n", ""))
		if err != nil || blob.GetSHA() != entry.GetSHA() || fmt.Sprintf("%x", sha1.Sum([]byte(fmt.Sprintf("blob %d\x00%s", len(content), content)))) != entry.GetSHA() || blob.GetEncoding() != "base64" || len(content) > domain.BootstrapMaxBytes || !utf8.Valid(content) || strings.ContainsRune(string(content), 0) {
			return domain.BootstrapSnapshot{}, domain.ErrRepositoryUnavailable
		}
		snapshot.Files[path] = string(content)
	}
	return snapshot, nil
}
