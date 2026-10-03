package github

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	gh "github.com/google/go-github/v74/github"

	"github.com/r59q/hub-rearranger/services/identity/internal/domain"
)

const bootstrapTitle = "Set up codex-thorough GitHub workflows"

func bootstrapBody(origin string, repo domain.Repository, user domain.User, proposal domain.BootstrapProposal) string {
	setup := origin + "/agents/bootstrap?" + url.Values{"repository": {repo.Owner + "/" + repo.Name}}.Encode()
	return fmt.Sprintf(`Installs the reviewed repository-local codex-thorough configuration and canonical GitHub-native workflow. Hub is optional after installation.

[Selected repository/profile setup](%s)

Reviewed by @%s at base %s; bootstrap digest %s.

This PR contains the profile catalog, assignment and no-write diagnostic workflows, offline validation workflow, pinned authorization/runtime helpers, installation documentation and proposed instruction additions. It enables no runner or credentials. Runtime writes stay split between the isolated read-only patch job and the hosted draft-PR publisher.

### Manual runner installation checklist

- [ ] Provision a dedicated restricted non-root Linux host/account for the approved private repository, with self-hosted, linux and hub-agent-codex labels. The upstream shared/public exception does not transfer.
- [ ] Install the reviewed checksum-pinned runtime, native Codex 0.159.3 and matching code-mode helper outside checkouts. Keep subscription authentication private on the host; create a disabled operator manifest.
- [ ] Verify workload filesystem/process/authentication isolation and disabled networking using the source-free preflight, with exact gpt-6.1-sol/high and no fallback.
- [ ] Supply the repository's root make check target and its required offline dependencies.
- [ ] Merge the reviewed configuration, allow Actions to create PRs, and dispatch the no-write profile diagnostic from the default branch as a current maintainer/admin.
- [ ] Verify fresh origin-bound AW-006 evidence for the exact current revision and policy; renew after every default-branch change and within 24 hours.
- [ ] Enable both the private operator manifest and HUB_AGENT_EXECUTION_ENABLED=verified only after verification.
- [ ] Use the documented GitHub issue-comment assignment. For recovery, rerun only the original authorize job and dependent jobs while preserving artifacts. PR review continuation remains AW-016.

See docs/agent-workflows.md in this PR for installation, rotation, revocation and recovery. No hidden Hub configuration is retained; removing Hub preserves the GitHub-owned flow.
`, setup, user.Login, proposal.BaseRevision, proposal.Digest)
}

func (w bootstrapWrite) findPullRequest(ctx context.Context, head string) (*gh.PullRequest, error) {
	options := &gh.PullRequestListOptions{State: "all", Head: w.repo.Owner + ":" + w.branch, ListOptions: gh.ListOptions{PerPage: 100}}
	requests, response, err := w.api.PullRequests.List(ctx, w.repo.Owner, w.repo.Name, options)
	if err != nil || response == nil || response.NextPage != 0 || len(requests) > 1 {
		return nil, domain.ErrBootstrapIncomplete
	}
	if len(requests) == 0 {
		return nil, nil
	}
	pr := requests[0]
	if pr.GetNumber() <= 0 || pr.GetUser().GetID() != w.user.ID || !pr.GetDraft() || pr.GetState() != "open" || pr.GetTitle() != bootstrapTitle || pr.GetBody() != w.body || pr.GetHead().GetRef() != w.branch || pr.GetHead().GetSHA() != head || !strings.EqualFold(pr.GetHead().GetRepo().GetFullName(), w.proposal.Repository) || pr.GetBase().GetRef() != w.proposal.DefaultBranch || !strings.EqualFold(pr.GetBase().GetRepo().GetFullName(), w.proposal.Repository) || pr.GetHTMLURL() != fmt.Sprintf("https://github.com/%s/pull/%d", w.proposal.Repository, pr.GetNumber()) {
		return nil, domain.ErrBootstrapConflict
	}
	return pr, nil
}

func (w bootstrapWrite) pullRequest(ctx context.Context, head string) (*gh.PullRequest, error) {
	pr, err := w.findPullRequest(ctx, head)
	if err != nil || pr != nil {
		if err == nil {
			err = w.guard(ctx)
		}
		return pr, err
	}
	if err := w.guard(ctx); err != nil {
		return nil, err
	}
	if _, err := w.currentBase(ctx); err != nil {
		return nil, err
	}
	ref, _, err := w.api.Git.GetRef(ctx, w.repo.Owner, w.repo.Name, "heads/"+w.branch)
	if err != nil || ref.GetObject().GetSHA() != head {
		return nil, domain.ErrBootstrapConflict
	}
	_, _, _ = w.api.PullRequests.Create(ctx, w.repo.Owner, w.repo.Name, &gh.NewPullRequest{
		Title: gh.Ptr(bootstrapTitle), Body: gh.Ptr(w.body), Head: gh.Ptr(w.branch), Base: gh.Ptr(w.proposal.DefaultBranch), Draft: gh.Ptr(true), MaintainerCanModify: gh.Ptr(false)})
	// Reconcile actual GitHub state even after success or a lost HTTP response.
	pr, err = w.findPullRequest(ctx, head)
	if err != nil {
		return nil, err
	}
	if pr == nil {
		return nil, domain.ErrBootstrapIncomplete
	}
	return pr, nil
}
