# Codex patch execution (AW-012)

This operator-installed runtime proposes a patch for an AW-011 assignment. It is
external to Hub and Docker Compose. The Go `ops/agent-intake/cmd/patch` verifier
owns GitHub reads through `go-github`; the Python launcher owns bounded source
materialization, Codex execution, validation, patch capture, and safe results.
Neither component writes branches, PRs, comments, or checks. The separate
[AW-013 hosted publisher](../agent-intake/README.md#separate-publication-aw-013)
owns publication and independently verifies current authorization and artifacts.

## Operator exception and trust boundary

On 2026-10-02 the operator explicitly selected the existing shared `addons`
runner/account for `r59q/hub-rearranger`. That named repository may run the
credentialed diagnostic and patch job even though it is public. Other public
repositories remain blocked. Default-branch, non-fork, current maintainer/admin,
canonical assignment/run, current policy, and runtime-evidence guards still apply.
The normal deployment target remains a restricted dedicated host/account.

Other jobs under the shared `r59q` account are part of the approved trust boundary:
they can change account-owned installed tooling or authentication outside these
workload sandboxes. The exception does not permit issue/source commands to read
host authentication, execute outside their sandbox, inherit ambient tools/config,
or use workload networking. A runner label selects a runner; it does not establish
account isolation or restrict repository access.

## Installation

The fixed installation directory is `~/.local/lib/hub-agent-execution`, with
private temporary work under `~/.local/state/hub-agent-execution`. Keep both outside
Actions checkouts. Files and parent directories must be owned by the operator
account or root and must not be group/world writable. The state directory requires
mode `0700`. The workflow pins the launcher checksum; the operator manifest pins
all native binaries, launcher modules, probes, schemas, and validator helpers.
The installed Python and its packages are operator-managed pinned dependencies.

Build `ops/agent-intake/cmd/patch` with the repository's pinned Go toolchain:

```sh
cd ops/agent-intake
go build -trimpath -o /tmp/agent-patch ./cmd/patch
```

Install the native Linux x86_64 **Codex 0.159.3** executable as `codex` in the
runtime directory, with the matching distribution’s native `codex-code-mode-host`
helper beside it. Pin both digests. Install `agent-patch`, `execution*.py`, `isolation_probe.py`,
`diagnose.py`, `profile_diagnostic.py`, and `result.schema.v1.json` beside it.
Preserve these canonical helper paths below the runtime directory:

- `ops/agent-intake/profile_policy.py` and `execution_policy.py`;
- `ops/agent-profiles/catalog.py`, `validate.py`, `diagnostics.py`, and `schema.v1.json`;
- `ops/private-runner/readiness.schema.v1.json` and `result.schema.v1.json`.

Create a `tools` Python virtual environment there with `python3 -m venv --copies`,
using the pinned `ops/agent-profiles/requirements-dev.txt`. Materialize any remaining
symlinks, including `tools/lib64`, as ordinary copies of their trusted local targets.
The workflow checks every installed entry with `find -perm /022`; symlinks normally
have mode `0777` and fail this check even when their targets are read-only.
Python 3.11+ and a Linux kernel with
Bubblewrap/user-namespace/seccomp support are required. No source dependencies
or provider credentials belong in that environment. Keep the existing cached
ChatGPT login in `~/.codex/auth.json`, owned by the runner account with mode
`0600`; do not copy it into the runtime, checkout, bundle, or artifacts. Global
`~/.codex/AGENTS.md`/`instructions.md` are unsupported and block startup.

Run the same installation guard as the workflow before generating the manifest:

```sh
test "$(id -u)" -ne 0
test -z "$(find "$HOME/.local/lib/hub-agent-execution" -xdev \( -perm /022 \) -print -quit)"
```

The source-free preflight below checks resolved-path ownership and permissions;
it does not replace this check of every installed entry. Verify the launcher's
SHA-256 against the reviewed workflow's pinned value as well.

Generate the **disabled** manifest from the installed reviewed files:

Replace `owner/repository` and `hub-agent-codex-01` below with the approved
repository and actual dedicated runner name. The shared `addons` name is allowed
only for the explicitly approved upstream repository described above.

```sh
python3 -I "$HOME/.local/lib/hub-agent-execution/execution_setup.py" \
  owner/repository hub-agent-codex-01 > "$HOME/.local/lib/hub-agent-execution/operator.json"
chmod 0600 "$HOME/.local/lib/hub-agent-execution/operator.json"
```

The manifest has no commands or credentials. Its repository, actual runner name,
CLI version, and exact file digests must match. `enabled` defaults to `false`;
never regenerate it merely to accept unexplained installed-file changes.

Run the fixed source-free local preflight:

```sh
python3 -I "$HOME/.local/lib/hub-agent-execution/execution_setup.py" \
  owner/repository hub-agent-codex-01 --preflight
```

It verifies installation permissions/digests, then checks the same restricted
filesystem/network policy used for source execution. Real host/auth/process
canaries must be unreadable, scratch/source access must work, and networking must
return enforcement errors rather than merely time out. The fresh PID namespace
has its own harmless `/proc/1`; the probe tests the actual host harness PID.
The fixed no-write subscription probe uses the exact launcher flags and verifies
`gpt-6.1-sol`/`high` from CLI session
headers with no fallback. It prints only allowlisted metadata. A local preflight
is **not** AW-006 GitHub evidence or assignment authorization.

## Enablement and execution

1. Review and publish the updated assignment and profile-diagnostic workflows on
   the approved repository's default branch. Add `hub-agent-codex` to the approved
   runner's custom labels; do not change unrelated runner registrations or labels.
2. Install the checksum-pinned AW-006 probes in the existing diagnostic directory,
   and make its `codex` executable refer to the verified execution CLI. Preserve
   the AW-001 operating/revocation procedure.
3. Dispatch `agent-profile-diagnostic.yml` as a current maintainer/admin. Confirm
   that the latest exact attempt publishes a verified `agent-readiness-v1-*`
   artifact for the current default-branch revision and exact CLI/model policy.
4. Only after local isolation and GitHub evidence are verified, set manifest
   `enabled: true` and repository variable `HUB_AGENT_EXECUTION_ENABLED=verified`.
   Both gates are required; changing the variable alone cannot enable execution.
5. Create a trusted assignment, or rerun only the `authorize` job and dependent
   jobs in its original canonical workflow. Avoid **Re-run all jobs** after source
   execution: the original proposal artifacts must remain available for recovery.
   The hosted `dispatch` gate independently reconstructs authority and readiness
   and reconciles all previous attempts. The runner repeats those checks before
   collecting pinned source/current permitted issue context and before execution.

The runner job has Contents/Actions/Issues **read** permissions and no checkout,
package installation, Git push, or GitHub write step. GitHub tokens reach only the
installed Go collection process. Codex and workload commands receive explicit
minimal environments, no GitHub/API/SSH/proxy variables, and no Actions credentials.
The auth-bearing harness keeps access to cached ChatGPT login; its tool commands
run under a separate restricted-read Bubblewrap/seccomp boundary with network off.
Only the reviewed native CLI and its matching code-mode helper are exposed from
the runtime installation; JavaScript orchestration exposes only the configured
tools, whose source commands retain the sandbox boundary.

Source materialization rejects path traversal, duplicates, oversized archives,
links, special files, and submodules. Repository `.codex`/`.agents` directories,
credential-like `.env` files, and instructions are omitted from disk. Permitted
repository `AGENTS.md` contents are explicitly supplied as bounded untrusted
context. Automatic instruction discovery is disabled. User config/rules, hooks,
plugins, apps, host skills, browser/computer/image tools, and subagents are disabled;
ambient managed restrictions may only cause a safe failure.
Codex's generic notice for the pinned host-skill-discovery flag is suppressed
with the documented `suppress_unstable_features_warning` setting. Other runtime
warnings still fail exact-policy verification.

Git metadata/index/config stays outside the workload's readable roots. The adapter
captures actual source changes, including new files, while honoring ignores for
new build output. Changes to workflows, runtime/profile/intake tooling,
instructions, credential/config paths, unsafe file types, and submodules are
rejected. Newly created project config is rejected before validation can load it.
The fixed `repository-check` runs `make check` in the same sandbox; dependencies
must already be available offline. Failed/unavailable validation remains explicit
on an otherwise ready proposal, following `draft-with-evidence` policy.

[`result.schema.v1.json`](result.schema.v1.json) is the closed output contract.
`agent-patch-v1-{attempt}` contains `result.json`, plus `proposal.patch` and
`summary.json` only for `ready`. Identity, exact requested/effective policy,
validation outcomes, and proposal SHA-256 digests are adapter-owned. Summary prose
is fixed; raw provider output, source bodies, environment dumps, and exceptions
never become logs or result metadata. Patches can contain repository source;
public-repository proposals must be public-safe. Retention is seven days.

## Recovery and revocation

The repository/comment concurrency group remains unchanged across duplicates and
reruns. Only AW-011's canonical earliest run may execute. Hosted-only skipped
attempts can resume; a previous successful runner attempt with a verified ready
artifact is reused without another Codex run. Recovery independently checks run,
attempt, assignment/base/profile identity, schema, origin, archive digest, and patch
and summary digests. The write job still has to authorize and publish it.

Use the original `authorize` job's individual rerun control so its dependent
jobs reauthorize and reconcile execution/publication while retaining earlier
artifacts. We observed full workflow reruns remove those artifacts, consistent
with the [support behavior reported here](https://github.com/orgs/community/discussions/17854).
Downloading a ZIP does not preserve its GitHub origin or restore a deleted
artifact. Retention limits and unique attempt names do not prevent this loss.

A failed/cancelled runner, incomplete jobs history, expired/missing artifact, or
ambiguous prior execution fails closed as `REPLAY_STATE_UNAVAILABLE`; it cannot
blindly re-execute. Inspect the original run and reconcile its actual side effects.
Use a new explicit assignment only after confirming the original workload is gone.
No proposal is pushed directly, so runner output alone cannot prove publication.

The hosted `report` job uses `always()` to publish a safe job summary and
`agent-execution-status-v1-*` even after ordinary failure/cancellation or missing
runner output. This status is observation, not authority. A force-cancelled Actions
run can prevent all reporting; its GitHub run conclusion remains the source of
truth, and reruns must reconcile history. Processes/descendants are killed on exit;
private work and raw output are removed, and transferred proposal files are cleaned
with an `always()` runner step. After runner loss, drain execution and remove any
leftover private work before recovery.

To revoke, remove `HUB_AGENT_EXECUTION_ENABLED`, set manifest `enabled: false`,
disable the current profile, and drain/cancel runner jobs. For credential revocation,
follow [README.md](README.md#rotation-revocation-and-recovery). Never rotate or copy
credentials through Hub/GitHub or restore a whole runner-home backup. After tool,
policy, CLI, or default-branch revision changes, review/reinstall digests and rerun
both preflight and the latest AW-006 diagnostic before enabling execution.

## Development and verification

Run `make runtime-check intake-check` and `make check`. Offline tests run on
GitHub-hosted runners with synthetic source, tokens, CLI processes, and controlled
HTTP/TLS servers. They require no model request, live GitHub access, or Codex login.
The implementation uses the [official permissions profiles](https://learn.chatgpt.com/docs/permissions)
and [non-interactive CLI](https://learn.chatgpt.com/docs/non-interactive-mode).
The notice setting follows the [configuration reference](https://learn.chatgpt.com/docs/config-file/config-reference).
Installation/local preflight alone cannot prove readiness. AW-012 was completed
with origin-verified AW-006 evidence and an independently verified controlled
Actions patch artifact; see [the task record](../../AGENTIC_WORKFLOWS_TASKS.md)
for the exact runs, revision, digest, and failed offline validation outcome.
This historical verification does not authorize future assignments or replace
fresh evidence for the current default-branch revision.
