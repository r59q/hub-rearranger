# Trusted assignment intake and publication (AW-011–AW-013)

This Agents-domain operations component runs the GitHub-hosted assignment intake,
read-only execution verification, and separate branch/draft-PR publisher. It lives
outside Docker Compose and owns no service, queue, database, or Codex authentication.
The workflow and GitHub objects remain usable without Hub.

The [assignment workflow](../../.github/workflows/agent-assignment.yml) uses hosted
`authorize` and `dispatch` jobs, an operator-installed read-only `patch` job, a
hosted `publish` job, and hosted safe reporting. See the
[execution guide](../private-runner/EXECUTION.md) for runner setup. Source execution
requires matching runtime evidence, isolation, and both operator gates. Publication
independently reconstructs authorization and verifies the resulting artifact; no
job output, receipt, or earlier authorization becomes a write grant.

## Ownership and dependencies

- `cmd/intake`, `cmd/patch`, and `cmd/publish` compose the reviewed commands and safe Actions output handling.
- `internal/domain` owns assignment validation, live authorization, policy
  matching, canonical-run selection, publication sequencing, and retry identity rules. It depends only
  on ports and plain models.
- `internal/infrastructure/github` uses pinned `go-github` for REST, pagination,
  current source/roles/revisions, workflow/artifact verification, Git objects, and publication reconciliation.
- `profile_policy.py` calls the existing AW-003 bounded parser and Draft 2020-12
  validator. It compares all execution fields, treating context/check lists as
  sets; it contains no independent profile schema. Go invokes it with `-I`, a
  fixed reviewed script path, bounded structured stdin, and no workflow token.
- `publication_policy.py` validates the canonical AW-012 result schema, originating
  invocation, exact policy/identity, patch digest, and structured summary digest.
- `internal/infrastructure/patch` calls `publication_patch.py` with a minimal
  environment and no workflow token. It shares AW-012's canonical source limits
  and protected paths, and uses Git's cached index application without executing
  or checking out patched code.
- `internal/report` emits fixed reason codes and recovery guidance. Raw GitHub,
  parser, subprocess, source-comment, and credential text never becomes a report.

Requirements: Go 1.25 and Python 3.11+ with the pinned combined tools in
[`requirements-dev.txt`](requirements-dev.txt). The repository-standard Go
formatter/static analysis and Python Ruff settings apply here.

## Intake and current authorization

Post the documented single-line command on an open issue:

```text
/agent assign codex-thorough@0123456789abcdef0123456789abcdef01234567 authority=branch-draft-pr
```

Replace the example SHA with a full lowercase Git commit containing the catalog
in the current default branch's history. The workflow accepts only
`issue_comment.created`; normal comments do not enter its jobs. Edited comments,
fresh PR assignments, bots, missing/deleted/closed sources, fork repositories,
renamed or mismatched repository IDs, malformed commands, and unapproved roles
fail closed. Source contents are never interpolated into shell code.

Intake re-fetches the comment and parent issue. The comment body, author ID/login,
parent issue, and creation/update timestamps must still match the original event.
It checks the actual requester's, original actor's, and rerun actor's current
`maintain`/`admin` role using `role_name`; the coarse `permission: write` is
insufficient. No claimed requester, receipt, or previous success grants authority.

The pinned SHA must be a default-branch ancestor. Both exact-SHA catalogs validate,
and both selected profiles must exist and be enabled. Material changes produce
`PROFILE_POLICY_CHANGED`; display-name/description changes and set ordering do
not. V1 dispatch permits only `codex-chatgpt-private-runner` contract v1 with the
fixed `hub-agent-codex` label. Other labels fail closed rather than selecting an
unapproved runner. Actual model compatibility and dedicated-runner readiness
remain AW-012 responsibilities.

Before publishing acceptance, intake rechecks source/roles and resolves the
default branch again. A moved branch requires fresh ancestry/current-policy
validation. Authorization is still a snapshot: AW-012 must reauthorize before
execution, and AW-013 before applying/publishing a patch.

## Identity, concurrency, and retries

Assignment identity is `<repository-id>:<request-comment-id>`. The workflow
`run-name` embeds this identity, and the top-level concurrency group uses those
same immutable IDs with `cancel-in-progress: false`. All future patch/publish
jobs must stay inside that group. GitHub's default concurrency behavior may
replace a pending duplicate, but cannot cancel the active assignment. Separate
comment IDs have separate groups and identities.

Intake verifies its current run against GitHub and reads the registered
`agent-assignment.yml` run history. It elects the earliest matching run ID,
independent of queue order. Only that canonical run may dispatch; duplicate
deliveries show a link to rerun it and create no extra receipt or invocation.
History is limited to the comment/run creation window, ten pages/1000 filtered
runs, and the exact workflow path, event, title, and repository ID. Truncated,
missing, or unverified history fails closed. This implementation targets
GitHub.com; GitHub Enterprise Server requires a reviewed URL/identity adapter.

The canonical run posts one acceptance comment containing a closed
`agent-intake:v1` block with assignment/run/attempt/profile/requester IDs and accepted
base SHA. Reruns update that same comment and retain the base while revalidating
current policy. Only receipts authored by the GitHub-verified
`github-actions[bot]` account are considered, and their metadata must match the
freshly authorized request and canonical run. Receipts are not execution
completion proof or an authority grant. Missing receipts on rerun, duplicate or
malformed bot records, rewritten ancestry, or inconsistent metadata fail closed.
Keep workflow history and receipts available; if provenance cannot be recovered,
post a new assignment comment rather than guessing prior state.

A failed/lost comment POST response is reconciled by reading GitHub once before
dispatch; no blind second POST is issued. A receipt created before a later
artifact-upload failure allows the original run to resume with its original
base. AW-012/AW-013 must separately reconcile completed execution/artifacts and
published branches/PRs before repeating those side effects. The intake receipt
alone must never be used to assert work is incomplete or authorize another write.

## Invocation artifact and GitHub-visible outcomes

Accepted/resumed intake uploads `agent-invocation-v1-<run-attempt>` containing
`invocation.json`, retained for seven days. Its contract version is `1` and
operation is `assignment`. It contains:

- repository ID/owner/name/default branch and source issue/request IDs/URLs;
- verified requester ID/login/current role, declared authority, profile ID and
  pinned/current policy revisions;
- validated execution policy, excluding arbitrary display prose;
- accepted `base_sha`, canonical workflow run ID, current attempt/URL, and
  acceptance receipt comment ID.

No source body, credential, prompt, command string, model transcript, or arbitrary
API error is included. The artifact is context, never a portable execution grant.
Future consumers must verify repository/workflow/run/attempt origin and current
authorization independently, rather than trusting uploaded metadata or a URL.

Rejection fails the intake step with a fixed GitHub Actions check/log/summary and
useful guidance; it does not post rejection comments for arbitrary actors.
Acceptance updates the source issue with verified identity/profile/authority and
links to the request and exact workflow attempt. It explicitly reports that
execution is gated independently by AW-012; a green intake indicates accepted input only.

The hosted authorization job has Contents read, Actions read, and Issues write
for its receipt. Dispatch and patch execution have Contents/Actions/Issues read.
Only the separate hosted publisher has Contents, Pull requests, Issues, and
Checks write plus Actions read; it has no model credential, App secret, or
Hub discovery token. Protect the default
branch and intake/validator/workflow code with maintainer review. The workflow
checks out its immutable `github.workflow_sha`, never the request's profile SHA,
and does not persist GitHub credentials in checkout configuration.

## Separate publication (AW-013)

The hosted publisher checks out only `github.workflow_sha` with persisted
credentials disabled. It builds the reviewed Go command and installs pinned
validation tools before the credentialed publication step. It never runs Codex,
repository recipes, package hooks from the proposal, or `git push`.

`cmd/publish` verifies the exact successful intake and patch job/attempt, immutable
artifact origin and archive digest, and the originating invocation. It verifies
current source/maintainer roles, canonical run ownership, revision ancestry, and
pinned/current profiles independently. A recovered patch must come from the one
verified prior successful attempt; skipped execution on the current attempt is
not itself proof that a proposal exists. The default branch must still equal the
accepted base. A moved base fails as `STALE_HEAD`; v1 never rebases or force-pushes.

The canonical AW-012 result schema and adapter-owned proposal/summary digests are
checked again. Source collection is pinned to the accepted commit. The patch is
applied only to a private Git index with explicit environment/config, hooks and
filters disabled. The adapter rejects protected paths, escapes, malformed/stale
patches, unsafe modes, symlinks, and submodules. Binary content, executable modes,
and deletions are supported. Limits remain 32 MiB compressed source, 128 MiB
expanded source, 8 MiB patch, 16 MiB per file, and at most 100 changed paths.
The original full base tree preserves untouched protected/omitted files.

Publication uses `go-github` Git-object APIs to create a deterministic commit and
then **create**, never update, `agent/codex-thorough/<repository-id>-<comment-id>`.
The commit binds the verified parent/tree, fixed author/date, assignment, original
proposal attempt/artifact, and patch digest. An existing ref must equal this
independently reconstructed commit. Current authorization/head is rechecked
between patch preparation, immutable object creation, branch creation, draft PR,
comment, check, and final confirmation.

One draft PR carries AW-002 `agent-assignment:v1` provenance plus
`agent-publication:v1` base/head/artifact/digest metadata, source/request/run links,
and explicit validation evidence. Recovery verifies the bot author, repository,
branch and immutable commit, exact metadata/body, draft/open state, and base.
An edited, closed, merged, moved, forked, or ambiguous PR fails closed; the
publisher cannot merge or update an unrelated branch or PR. The issue receives
one concise proposal link. `Agent proposal / repository-check` is successful only
when validation passed; failed/unavailable validation publishes a **neutral** check
and remains explicit on the draft under `draft-with-evidence`.

The concurrency group remains the original repository/comment identity across
all jobs and attempts. The existing verified intake receipt gains optional,
closed `publication` intent (artifact ID, attempt, head, comment/check started
flags), preserved by intake on reruns. Before non-idempotent comment/check POSTs,
the publisher records intent. It always reconciles actual bot/app-owned objects
by identity and content; a started stage with a missing/mismatched object is
`REPLAY_STATE_UNAVAILABLE`, never permission to repeat that POST. A lost response
can be recovered by verified GitHub state. Missing branch/PR creation can resume;
existing objects cannot be overwritten. Force cancellation or ambiguous history
requires operator reconciliation of the original run before a new assignment.

After confirming the branch, draft PR, comment and check, `publication-status.json`
records version 1, assignment/run/attempt, `published`/`DRAFT_PR_PUBLISHED`, and the
branch/head/PR identity. It is uploaded as `agent-publication-status-v1-<attempt>`.
Ordinary command failures record `incomplete` with the specific allowlisted reason
code and Actions run/attempt identity. An `always()` step supplies generic
`PUBLICATION_INCOMPLETE` evidence if the command could not write its status file.
The final hosted execution report links the published PR or reports incomplete/skipped
publication. No raw source, patch, subprocess output, or exception is reported.

For live verification after merging this workflow:

1. Permit GitHub Actions to create pull requests in the approved repository's
   [Actions settings](https://docs.github.com/en/repositories/managing-your-repositorys-settings-and-features/enabling-features-for-your-repository/managing-github-actions-settings-for-your-repository).
   Keep default workflow permissions minimal; the publisher declares its scopes.
   No extra long-lived write credential or automatic PR approval is needed.
2. Rebuild/reinstall the reviewed `agent-patch` binary and pin its new digest in
   the operator manifest: the optional receipt intent must be understood by both
   hosted and operator-installed verifiers. Keep execution disabled while updating.
3. Run the exact installation guard and source-free preflight, obtain fresh
   origin-verified AW-006 evidence for the merged default-branch revision, and
   re-enable both execution gates. Old artifacts cannot authorize a stale base.
4. Use a new controlled documentation-only assignment. Verify the resulting
   branch, draft provenance, actual diff, validation check and issue link; then
   rerun the original assignment and confirm no duplicate execution/publication.

Workflow-code repairs do not change an old run's immutable checkout revision.
After merging a repair, renew readiness for the new default-branch revision.
Use a new assignment only after reconciling the original run's side effects and
confirming its private workload is gone; stale proposals cannot be rebased by this
publisher. GitHub's Git API returns commit messages without a final newline, so
the deterministic message uses that canonical form for exact object verification.

GitHub's documented [token event behavior](https://docs.github.com/en/actions/how-tos/write-workflows/choose-when-workflows-run/trigger-a-workflow)
can require human approval for PR workflows triggered by `GITHUB_TOKEN`; other
ordinary token-triggered events do not start new workflows. Do not treat missing
or approval-pending repository CI as passed. The publisher's check reports only
the recorded offline validation and does not approve workflows or merge the PR.

## Development and verification

From the repository root, install the existing combined tools and run:

```sh
python3 -m venv ops/private-runner/.venv
ops/private-runner/.venv/bin/python -m pip install -r ops/agent-intake/requirements-dev.txt
make intake-check
make check
```

`PROFILE_PYTHON` selects another pinned Python environment. Standalone Go tests
also accept absolute `INTAKE_PYTHON`; the default is the root private-runner venv.
For race checks: `cd ops/agent-intake && go test -race ./...`.

Tests use domain fakes plus a controlled HTTP server with real `go-github` and
the actual Python validator. They cover exact command/event parsing, source and
permission revocation, pinned/current policy, provenance/history limits,
pagination, simultaneous duplicate deliveries, rerun/base stability, forged
receipts, ambiguous publication, cancellation, and safe output. Workflow checks
verify event/concurrency/permissions/checkout/artifact wiring. No test contacts
GitHub, accesses Codex authentication, invokes a live runner, or writes to a real
repository. `make check` and GitHub-hosted runtime/application checks include this
component; Docker Compose is unchanged because this is external workflow tooling.

Protocol references: GitHub's [issue-comment event](https://docs.github.com/en/actions/reference/workflows-and-actions/events-that-trigger-workflows#issue_comment),
[workflow run API](https://docs.github.com/en/rest/actions/workflow-runs), and
[concurrency behavior](https://docs.github.com/en/actions/concepts/workflows-and-actions/concurrency).
