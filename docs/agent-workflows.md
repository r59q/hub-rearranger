# Install and use codex-thorough

This repository-local flow accepts an issue assignment, proposes an isolated
patch, and publishes a dedicated branch and draft PR through GitHub Actions.
GitHub owns profiles, requests, runs, evidence and results. Hub is optional.
Trusted PR review continuation is configured in the profile but is not yet
implemented (AW-016); post no follow-up expecting automatic branch updates.

## Review the bootstrap package

Review `.github/agent-profiles.yml`, `agent-assignment.yml`,
`agent-profile-diagnostic.yml`, `agent-bootstrap-checks.yml`, the proposed
`AGENTS.md` block, and the accompanying `ops` helpers before merging. Actions and
Go/Python dependencies use the canonical reviewed pins. The catalog, workflows,
schemas, authorization helpers and launcher form one versioned installation;
do not install only the YAML files or replace helpers with independent copies.

Generated execution and diagnostic workflows require a private, non-fork
repository and its default branch. A public installation needs separate explicit
operator approval and a reviewed policy change; the named shared-runner exception
for the upstream Hub repository is not granted to installations. Keep Actions'
default workflow permissions minimal. Only the separate hosted publisher has
branch/PR write permission, with no Codex authentication and no proposal-source
execution. The patch job has repository-read permission and cannot push.

The `Agent bootstrap checks` workflow validates the portable package on
GitHub-hosted runners with offline tests and no Codex authentication. The
repository must also provide its own root `make check` target for workload
validation, with required tools and dependencies already available offline.
The bootstrap does not overwrite the repository's Makefile or provision tooling.
Missing or failed checks are reported honestly under `draft-with-evidence`.

## Manual runner installation checklist

1. Provision a dedicated, non-root Linux runner host and service account,
   restricted to this approved private repository or runner group. Add the custom
   `hub-agent-codex` label alongside `self-hosted` and `linux`. A label selects a
   runner; it does not restrict repository access or prove account isolation.
2. Follow [the runner guide](../ops/private-runner/README.md) for protected
   subscription-backed ChatGPT authentication, rotation and revocation. Keep
   `~/.codex/auth.json` on the host with mode `0600`; never put authentication,
   tokens, local configuration, or an operator manifest in a checkout or artifact.
3. Follow [the execution installation guide](../ops/private-runner/EXECUTION.md)
   to install the reviewed Codex 0.159.3 native binary and code-mode helper,
   launcher, `agent-patch` verifier, schemas and validator helpers outside Actions
   checkouts. Use this installation's repository and actual dedicated runner name
   when generating `operator.json`; its `enabled` value must start as `false`.
   The upstream `addons` exception does not authorize an installation.
4. Install the checksum-pinned `diagnose.py` and `profile_diagnostic.py` together
   at `~/.local/lib/hub-agent-runtime/`, with the reviewed native CLI. Verify
   ownership, permissions and every pinned digest. The launcher Python environment
   requires ordinary copies (`python3 -m venv --copies`); materialize remaining
   symlinks as described in the execution guide. Work state requires mode `0700`.
5. Run the source-free launcher preflight. Prove that workload commands cannot
   read host authentication/process/configuration or use networking. Verify exact
   `gpt-6.1-sol` with `high` reasoning and no fallback. A local preflight, a runner
   label, or a green workflow alone is not GitHub readiness evidence.
6. Merge the reviewed package, permit Actions to create pull requests in the
   repository's Actions settings, and manually dispatch `Agent profile diagnostic`
   from the default branch as a current maintainer/admin. Independently verify
   the latest `agent-readiness-v1-<attempt>` artifact against its workflow/job,
   run/attempt, current full commit SHA, digests, exact policy and successful
   isolation preflight. [The evidence contract](../ops/private-runner/READINESS.md)
   defines origin and freshness rules. Evidence expires after 24 hours; any new
   default-branch commit requires a new diagnostic.
7. Only after verification, enable the operator manifest privately and set the
   repository variable `HUB_AGENT_EXECUTION_ENABLED` to `verified`. Both gates
   are required. The generator never enables execution or registers a runner.

## Assign an issue from GitHub

As a current repository maintainer/admin, create this single-line issue comment:

```text
/agent assign codex-thorough@0123456789abcdef0123456789abcdef01234567 authority=branch-draft-pr
```

Replace the example with the reviewed full 40-character default-branch commit
SHA containing the profile. Use an open issue; edited comments and fresh PR
assignments are not supported by this initial profile. GitHub supplies the source
and requester identity. Intake rechecks current source, requester/rerun actor
roles, enabled pinned/current profile policies and revision ancestry. A receipt
or readiness status is context, never a reusable authorization grant.

The resulting draft PR records source issue/comment, profile/revision, authority,
branch/head, originating run/attempt and validation evidence. Follow the issue
update and GitHub check to review it. Workflow success or a published draft does
not mean `make check` passed. Only the recorded validation outcome establishes
that. Authority permits a dedicated branch and draft PR, never merge, release,
secrets, permissions, instructions or workflow/profile/runtime changes.

## Recovery, updates and removal

For publication recovery, use the original individual `authorize` job's rerun
control and its dependent jobs. Avoid **Re-run all jobs** after source execution:
it can remove the original artifacts, which must retain their verified origin.
The workflow reauthorizes and reconciles the existing patch, branch, PR, issue
update and check; it cannot blindly execute again, overwrite refs or force-push.
Missing or ambiguous original evidence fails closed. Reconcile original side
effects before posting a new assignment; saved ZIPs or PR bodies are not proof.

Disable the repository variable and operator manifest before changing installed
runtime code, revoking access or replacing a runner. Reinstall reviewed pins,
recheck isolation and obtain fresh matching diagnostic evidence before enabling
again. Never regenerate a manifest to accept unexplained runtime changes.
Review upgrades as repository-local diffs; preserve existing profile policy and
unrelated instructions. Merge the package only after reviewing every change.

Removing Hub leaves these GitHub-native controls working. To remove execution,
disable both gates, revoke runner authentication/registration, and remove the
repository workflows/profile/helpers through a reviewed change. Retain the
GitHub issue/PR/check history and evidence required for reconciliation.
