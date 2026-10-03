package github

import (
	"context"
	"crypto/sha1"
	"fmt"
	"io/fs"
	"regexp"
	"strings"
	"unicode/utf8"

	gh "github.com/google/go-github/v74/github"

	"github.com/r59q/hub-rearranger/services/identity/internal/domain"
)

var bootstrapSHA = regexp.MustCompile(`^[0-9a-f]{40}$`)
var bootstrapDigest = regexp.MustCompile(`^[0-9a-f]{64}$`)
var bootstrapLogin = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]{0,38}$`)

func validBootstrapChanges(changes []domain.BootstrapChange) bool {
	if len(changes) == 0 || len(changes) > 256 {
		return false
	}
	paths, bytes := map[string]bool{}, 0
	for _, change := range changes {
		path := change.Path
		if !fs.ValidPath(path) || strings.Contains(path, "\\") || paths[path] || !utf8.ValidString(change.Content) || strings.ContainsRune(change.Content, 0) || len(change.Content) > 1<<20 || (change.BaseSHA != "" && !bootstrapSHA.MatchString(change.BaseSHA)) {
			return false
		}
		allowed := path == "AGENTS.md" || path == "docs/agent-workflows.md" || path == ".github/agent-profiles.yml" || path == ".github/actionlint.yaml" || path == ".github/workflows/agent-assignment.yml" || path == ".github/workflows/agent-profile-diagnostic.yml" || path == ".github/workflows/agent-bootstrap-checks.yml"
		for _, prefix := range []string{"ops/agent-intake/", "ops/agent-profiles/", "ops/private-runner/"} {
			if strings.HasPrefix(path, prefix) {
				name := strings.TrimPrefix(path, prefix)
				allowed = strings.HasSuffix(name, ".go") || strings.HasSuffix(name, ".py") || strings.HasSuffix(name, ".schema.v1.json") || name == "schema.v1.json" || name == "go.mod" || name == "go.sum" || name == "requirements-dev.txt" || name == "pyproject.toml" || name == "README.md" || name == "READINESS.md" || name == "EXECUTION.md" || name == ".gitignore" || name == "authorize.mjs" || name == "test_authorize.mjs"
			}
		}
		if !allowed {
			return false
		}
		paths[path] = true
		bytes += len(change.Content)
	}
	for path := range paths {
		parts := strings.Split(path, "/")
		for i := 1; i < len(parts); i++ {
			if paths[strings.Join(parts[:i], "/")] {
				return false
			}
		}
	}
	return bytes <= 4<<20
}

type bootstrapEntry struct{ SHA, Mode, Type string }

func bootstrapEntries(tree *gh.Tree) (map[string]bootstrapEntry, error) {
	if tree == nil || tree.GetTruncated() {
		return nil, domain.ErrBootstrapIncomplete
	}
	entries := map[string]bootstrapEntry{}
	for _, entry := range tree.Entries {
		if entry.GetType() == "tree" {
			continue
		}
		if _, duplicate := entries[entry.GetPath()]; duplicate {
			return nil, domain.ErrBootstrapConflict
		}
		entries[entry.GetPath()] = bootstrapEntry{entry.GetSHA(), entry.GetMode(), entry.GetType()}
	}
	return entries, nil
}

func (w bootstrapWrite) verifyHead(ctx context.Context, head string, base *gh.Commit) error {
	commit, _, err := w.api.Git.GetCommit(ctx, w.repo.Owner, w.repo.Name, head)
	if err != nil {
		return domain.ErrBootstrapIncomplete
	}
	email := fmt.Sprintf("%d+%s@users.noreply.github.com", w.user.ID, w.user.Login)
	if commit.GetSHA() != head || len(commit.Parents) != 1 || commit.Parents[0].GetSHA() != w.proposal.BaseRevision || commit.GetMessage() != bootstrapMessage(w.proposal) || commit.GetAuthor().GetName() != w.user.Login || commit.GetAuthor().GetEmail() != email || commit.GetCommitter().GetName() != w.user.Login || commit.GetCommitter().GetEmail() != email {
		return domain.ErrBootstrapConflict
	}
	expected, err := w.expectedEntries(ctx, base)
	if err != nil {
		return err
	}
	after, _, err := w.api.Git.GetTree(ctx, w.repo.Owner, w.repo.Name, commit.GetTree().GetSHA(), true)
	if err != nil {
		return domain.ErrBootstrapIncomplete
	}
	actual, err := bootstrapEntries(after)
	if err != nil {
		return err
	}
	if len(expected) != len(actual) {
		return domain.ErrBootstrapConflict
	}
	for path, entry := range expected {
		if actual[path] != entry {
			return domain.ErrBootstrapConflict
		}
	}
	return nil
}

func (w bootstrapWrite) expectedEntries(ctx context.Context, base *gh.Commit) (map[string]bootstrapEntry, error) {
	before, _, err := w.api.Git.GetTree(ctx, w.repo.Owner, w.repo.Name, base.GetTree().GetSHA(), true)
	if err != nil {
		return nil, domain.ErrBootstrapIncomplete
	}
	expected, err := bootstrapEntries(before)
	if err != nil {
		return nil, err
	}
	for _, change := range w.proposal.Changes {
		entry, exists := expected[change.Path]
		if (change.BaseSHA == "" && exists) || (change.BaseSHA != "" && (!exists || entry.SHA != change.BaseSHA || entry.Mode != "100644" || entry.Type != "blob")) {
			return nil, domain.ErrBootstrapStale
		}
		parts := strings.Split(change.Path, "/")
		for i := 1; i < len(parts); i++ {
			if _, exists := expected[strings.Join(parts[:i], "/")]; exists {
				return nil, domain.ErrBootstrapConflict
			}
		}
		expected[change.Path] = bootstrapEntry{bootstrapEntryForContent(change.Content), "100644", "blob"}
	}
	return expected, nil
}

func bootstrapEntryForContent(content string) string {
	return fmt.Sprintf("%x", sha1.Sum([]byte(fmt.Sprintf("blob %d\x00%s", len(content), content))))
}
