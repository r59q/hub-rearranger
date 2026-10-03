package github

import (
	"context"
	"fmt"
	"strings"

	gh "github.com/google/go-github/v74/github"

	"github.com/r59q/hub-rearranger/services/identity/internal/domain"
)

type BootstrapPublisher struct {
	API    *gh.Client
	Origin string
}

type bootstrapWrite struct {
	api      *gh.Client
	repo     domain.Repository
	user     domain.User
	proposal domain.BootstrapProposal
	branch   string
	body     string
	guard    func(context.Context) error
}

func (p BootstrapPublisher) Publish(ctx context.Context, token string, repo domain.Repository, user domain.User, proposal domain.BootstrapProposal, guard func(context.Context) error) (domain.BootstrapResult, error) {
	if !bootstrapSHA.MatchString(proposal.BaseRevision) || !bootstrapDigest.MatchString(proposal.Digest) || guard == nil || !validBootstrapChanges(proposal.Changes) || user.ID <= 0 || !bootstrapLogin.MatchString(user.Login) {
		return domain.BootstrapResult{}, domain.ErrBootstrapConflict
	}
	w := bootstrapWrite{api: p.API.WithAuthToken(token), repo: repo, user: user, proposal: proposal, guard: guard,
		branch: fmt.Sprintf("hub-bootstrap/codex-thorough-%d-%s-%s", user.ID, proposal.BaseRevision[:12], proposal.Digest)}
	w.body = bootstrapBody(p.Origin, repo, user, proposal)
	base, err := w.currentBase(ctx)
	if err != nil {
		return domain.BootstrapResult{}, err
	}
	if _, err := w.expectedEntries(ctx, base); err != nil {
		return domain.BootstrapResult{}, err
	}
	ref, response, err := w.api.Git.GetRef(ctx, repo.Owner, repo.Name, "heads/"+w.branch)
	if err != nil && !missing(response) {
		return domain.BootstrapResult{}, domain.ErrBootstrapIncomplete
	}
	if ref == nil && err == nil {
		return domain.BootstrapResult{}, domain.ErrBootstrapIncomplete
	}
	if ref == nil {
		if err := w.guard(ctx); err != nil {
			return domain.BootstrapResult{}, err
		}
		if _, err := w.currentBase(ctx); err != nil {
			return domain.BootstrapResult{}, err
		}
		entries := make([]*gh.TreeEntry, 0, len(proposal.Changes))
		for _, change := range proposal.Changes {
			entries = append(entries, &gh.TreeEntry{Path: gh.Ptr(change.Path), Mode: gh.Ptr("100644"), Type: gh.Ptr("blob"), Content: gh.Ptr(change.Content)})
		}
		tree, _, err := w.api.Git.CreateTree(ctx, repo.Owner, repo.Name, base.GetTree().GetSHA(), entries)
		if err != nil || !bootstrapSHA.MatchString(tree.GetSHA()) {
			return domain.BootstrapResult{}, domain.ErrBootstrapIncomplete
		}
		if err := w.guard(ctx); err != nil {
			return domain.BootstrapResult{}, err
		}
		author := &gh.CommitAuthor{Name: gh.Ptr(user.Login), Email: gh.Ptr(fmt.Sprintf("%d+%s@users.noreply.github.com", user.ID, user.Login))}
		commit, _, err := w.api.Git.CreateCommit(ctx, repo.Owner, repo.Name, &gh.Commit{Message: gh.Ptr(bootstrapMessage(proposal)),
			Tree: &gh.Tree{SHA: tree.SHA}, Parents: []*gh.Commit{{SHA: gh.Ptr(proposal.BaseRevision)}}, Author: author, Committer: author}, nil)
		if err != nil || !bootstrapSHA.MatchString(commit.GetSHA()) {
			return domain.BootstrapResult{}, domain.ErrBootstrapIncomplete
		}
		// Independently verify the proposed commit/tree before exposing a branch.
		if err := w.verifyHead(ctx, commit.GetSHA(), base); err != nil {
			return domain.BootstrapResult{}, err
		}
		if err := w.guard(ctx); err != nil {
			return domain.BootstrapResult{}, err
		}
		if _, err := w.currentBase(ctx); err != nil {
			return domain.BootstrapResult{}, err
		}
		ref, _, err = w.api.Git.CreateRef(ctx, repo.Owner, repo.Name, &gh.Reference{Ref: gh.Ptr("refs/heads/" + w.branch), Object: &gh.GitObject{SHA: commit.SHA}})
		if err != nil {
			// A lost response or concurrent creation is reconciled, never retried.
			ref, _, err = w.api.Git.GetRef(ctx, repo.Owner, repo.Name, "heads/"+w.branch)
			if err != nil {
				return domain.BootstrapResult{}, domain.ErrBootstrapIncomplete
			}
		}
	}
	if ref.GetRef() != "refs/heads/"+w.branch || ref.GetObject().GetType() != "commit" || !bootstrapSHA.MatchString(ref.GetObject().GetSHA()) {
		return domain.BootstrapResult{}, domain.ErrBootstrapConflict
	}
	head := ref.GetObject().GetSHA()
	if err := w.verifyHead(ctx, head, base); err != nil {
		return domain.BootstrapResult{}, err
	}
	pr, err := w.pullRequest(ctx, head)
	if err != nil {
		return domain.BootstrapResult{}, err
	}
	if err := w.guard(ctx); err != nil {
		return domain.BootstrapResult{}, err
	}
	if _, err := w.currentBase(ctx); err != nil {
		return domain.BootstrapResult{}, err
	}
	current, _, err := w.api.Git.GetRef(ctx, repo.Owner, repo.Name, "heads/"+w.branch)
	if err != nil || current.GetObject().GetSHA() != head {
		return domain.BootstrapResult{}, domain.ErrBootstrapConflict
	}
	if err := w.verifyHead(ctx, head, base); err != nil {
		return domain.BootstrapResult{}, err
	}
	return domain.BootstrapResult{Repository: repo.Owner + "/" + repo.Name, Branch: w.branch, HeadSHA: head,
		PullRequestNumber: pr.GetNumber(), PullRequestURL: pr.GetHTMLURL()}, nil
}

func missing(response *gh.Response) bool { return response != nil && response.StatusCode == 404 }

func bootstrapMessage(proposal domain.BootstrapProposal) string {
	return "Set up codex-thorough GitHub workflows\n\nHub bootstrap review: " + proposal.Digest
}

func (w bootstrapWrite) currentBase(ctx context.Context) (*gh.Commit, error) {
	repository, _, err := w.api.Repositories.Get(ctx, w.repo.Owner, w.repo.Name)
	if err != nil {
		return nil, domain.ErrBootstrapIncomplete
	}
	if repository.GetID() <= 0 || !strings.EqualFold(repository.GetFullName(), w.proposal.Repository) || repository.GetArchived() || repository.GetDisabled() || repository.GetDefaultBranch() != w.proposal.DefaultBranch {
		return nil, domain.ErrBootstrapStale
	}
	branch, _, err := w.api.Repositories.GetBranch(ctx, w.repo.Owner, w.repo.Name, w.proposal.DefaultBranch, 0)
	if err != nil || branch.GetCommit().GetSHA() != w.proposal.BaseRevision {
		return nil, domain.ErrBootstrapStale
	}
	commit, _, err := w.api.Git.GetCommit(ctx, w.repo.Owner, w.repo.Name, w.proposal.BaseRevision)
	if err != nil || commit.GetSHA() != w.proposal.BaseRevision || !bootstrapSHA.MatchString(commit.GetTree().GetSHA()) {
		return nil, domain.ErrBootstrapIncomplete
	}
	return commit, nil
}
