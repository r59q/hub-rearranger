package github

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"

	gh "github.com/google/go-github/v74/github"
	"github.com/r59q/hub-rearranger/ops/agent-intake/internal/domain"
)

func patchDigest(proposal domain.Proposal) string {
	digest := sha256.Sum256(proposal.Patch)
	return hex.EncodeToString(digest[:])
}

func publicationMessage(input domain.Invocation, proposal domain.Proposal) string {
	// GitHub's Git API omits the final newline when returning commit messages.
	// Use that canonical form so strict object verification remains exact.
	return fmt.Sprintf("Propose issue #%d for agent assignment %s\n\nProfile: %s@%s\nBase: %s\nProposal: %d/%d artifact %d\nPatch-SHA256: %s", input.IssueNumber, input.AssignmentID, input.ProfileID, input.ProfileRevision, input.BaseSHA, input.RunID, proposal.Attempt, proposal.ArtifactID, patchDigest(proposal))
}

// CommitProposal creates immutable Git objects only. The original full base tree
// preserves untouched protected/omitted source. No patched recipe ever executes.
func (c *Client) CommitProposal(ctx context.Context, input domain.Invocation, proposal domain.Proposal, changes []domain.FileChange) (string, error) {
	repo := input.Repository
	base, _, err := c.api.Git.GetCommit(ctx, repo.Owner, repo.Name, input.BaseSHA)
	if err != nil || base.GetSHA() != input.BaseSHA || !domain.ValidSHA(base.GetTree().GetSHA()) {
		return "", domain.HistoryUnavailable
	}
	entries := make([]*gh.TreeEntry, 0, len(changes))
	for _, change := range changes {
		if change.Mode != "100644" && change.Mode != "100755" {
			return "", domain.ProtectedChange
		}
		entry := &gh.TreeEntry{Path: gh.Ptr(change.Path), Mode: gh.Ptr(change.Mode), Type: gh.Ptr("blob")}
		if !change.Delete {
			blob, _, err := c.api.Git.CreateBlob(ctx, repo.Owner, repo.Name, &gh.Blob{Content: gh.Ptr(base64.StdEncoding.EncodeToString(change.Content)), Encoding: gh.Ptr("base64")})
			if err != nil || !domain.ValidSHA(blob.GetSHA()) {
				return "", domain.Unavailable
			}
			entry.SHA = blob.SHA
		}
		entries = append(entries, entry)
	}
	tree, _, err := c.api.Git.CreateTree(ctx, repo.Owner, repo.Name, base.GetTree().GetSHA(), entries)
	if err != nil || !domain.ValidSHA(tree.GetSHA()) || tree.GetSHA() == base.GetTree().GetSHA() {
		return "", domain.HistoryUnavailable
	}
	author := &gh.CommitAuthor{Name: gh.Ptr("github-actions[bot]"), Email: gh.Ptr("41898282+github-actions[bot]@users.noreply.github.com"), Date: &gh.Timestamp{Time: proposal.Created.UTC()}}
	message := publicationMessage(input, proposal)
	commit, _, err := c.api.Git.CreateCommit(ctx, repo.Owner, repo.Name, &gh.Commit{Message: gh.Ptr(message), Tree: &gh.Tree{SHA: tree.SHA}, Parents: []*gh.Commit{{SHA: gh.Ptr(input.BaseSHA)}}, Author: author, Committer: author}, nil)
	if err != nil || !domain.ValidSHA(commit.GetSHA()) {
		return "", domain.Unavailable
	}
	// Re-read to bind recovery to actual Git objects, not editable PR metadata.
	stored, _, err := c.api.Git.GetCommit(ctx, repo.Owner, repo.Name, commit.GetSHA())
	if err != nil || stored.GetSHA() != commit.GetSHA() || !sameProposalCommit(stored, input.BaseSHA, tree.GetSHA(), message, author) {
		return "", domain.HistoryUnavailable
	}
	return commit.GetSHA(), nil
}

func sameProposalCommit(stored *gh.Commit, base, tree, message string, author *gh.CommitAuthor) bool {
	parentMatches := len(stored.Parents) == 1 && stored.Parents[0].GetSHA() == base
	objectMatches := stored.GetTree().GetSHA() == tree && stored.GetMessage() == message
	return parentMatches && objectMatches && sameProposalAuthor(stored.GetAuthor(), author) && sameProposalAuthor(stored.GetCommitter(), author)
}

func sameProposalAuthor(actual, expected *gh.CommitAuthor) bool {
	return actual.GetName() == expected.GetName() && actual.GetEmail() == expected.GetEmail() && actual.GetDate().Equal(expected.GetDate())
}
