# Trusted assignment intake (AW-011)

This Agents-domain operations component implements the GitHub-hosted intake for
new issue assignments. It lives outside Docker Compose and owns no service,
queue, database, Codex authentication, checkout execution, branch, or PR. The
workflow and GitHub objects remain usable without Hub.

The [assignment workflow](../../.github/workflows/agent-assignment.yml) is
implemented locally. Publishing it to a repository's protected default branch
enables intake; no live workflow was installed or triggered during this task.
AW-012 adds a read-only execution verifier and an operator-installed patch job;
see [the execution guide](../private-runner/EXECUTION.md). Execution remains gated
on matching runtime evidence, isolation, and explicit operator enablement.
The hosted `dispatch` job reports `RUNNER_NOT_READY`
on a GitHub-hosted runner and never invokes Codex or a self-hosted runner.
AW-013 adds the separate branch/draft-PR publisher.

## Ownership and dependencies

- `cmd/intake` composes the trusted command and safe Actions output handling.
- `internal/domain` owns assignment validation, live authorization, policy
  matching, canonical-run selection, and retry identity rules. It depends only
  on ports and plain models.
- `internal/infrastructure/github` uses pinned `go-github` for REST, pagination,
  current source/roles/revisions, workflow history, and receipt reconciliation.
- `profile_policy.py` calls the existing AW-003 bounded parser and Draft 2020-12
  validator. It compares all execution fields, treating context/check lists as
  sets; it contains no independent profile schema. Go invokes it with `-I`, a
  fixed reviewed script path, bounded structured stdin, and no workflow token.
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

The hosted authorization job has only Contents read, Actions read, and Issues
write for this receipt. The dispatcher gate has no GitHub permissions. There is
no Contents write, model credential, App secret, discovery token, or repository
workflow-variable switch that enables source execution. Protect the default
branch and intake/validator/workflow code with maintainer review. The workflow
checks out its immutable `github.workflow_sha`, never the request's profile SHA,
and does not persist GitHub credentials in checkout configuration.

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
