# Private Codex runner diagnostic (AW-001)

This external runtime spike belongs to repository operations. It verifies a
subscription-backed Codex CLI on the `addons` GitHub Actions runner.
It is not a Hub service and has no API, checkout, GitHub writes, or Compose entry.

**GitHub Actions verification passed.** This repository is public. On
2026-09-28, the operator allowed this public test
provided the workflow verifies that the triggering GitHub user is a current
repository maintainer. The operator also relaxed host/account isolation for
this spike; the existing shared account is permitted temporarily.

SSH inspection on 2026-09-28 confirmed:

- `addons` runs Linux x86_64 and Python 3.14.7. Its repository-scoped Hub runner
  service is active and registered to `r59q/hub-rearranger`.
- Codex 0.158.0 is installed at
  `/home/r59q/.nvm/versions/node/v26.10.0/bin/codex` and is on the runner service
  PATH, although it is absent from the non-interactive SSH PATH.
- The service account is `r59q`, shared with an active `better-recipes` runner.
  It belongs to the `docker` and `wheel` groups. This is covered by the temporary
  operator-approved isolation exception.
- A suppressed-output `codex login status` check confirmed cached ChatGPT
  authentication. The auth file is owned by the runner user with mode `0600`.
  No credential contents were inspected or copied. After the isolation
  exception, a fixed live subscription request also passed over SSH.
- The probe is installed at `~/.local/lib/hub-agent-runtime/diagnose.py` with
  mode `0600`, alongside a `codex` symlink to the installed native executable.
  This avoids the Node shim's dependency on an interactive NVM PATH and requires
  no sudo. The workflow pins the installed probe's SHA-256.
- Running the installed probe over SSH passed all three stages: Codex version,
  cached ChatGPT login, and live subscription request. The seven offline tests,
  Ruff formatting/lint, and actionlint also pass after the installation change.

The workflow was installed on the default branch at commit
`e7711ffcff53d6ab88e0592c60d2291cf23b9e82`. The
[diagnostic run](https://github.com/r59q/hub-rearranger/actions/runs/36456987109)
passed its GitHub-hosted permission job and its `addons` probe job. The
[offline checks](https://github.com/r59q/hub-rearranger/actions/runs/36456987068)
also passed. The unrelated runner was unchanged.

## Runner security contract

**Temporary exception (2026-09-28):** the operator approved using the existing
shared host and service account for this diagnostic, and approved a public
repository when current maintainer permission is checked before scheduling the
credentialed job. Other jobs under `r59q`
are consequently part of its trust boundary. The user-owned probe checksum
detects accidental changes but cannot protect against a compromised shared
account. The dedicated host/account requirements below remain the target before
expanding execution beyond this spike. No fork or pull request event may enter
this workflow.

- Register `addons` only with this approved repository, or a runner group
  restricted to it. The workflow first runs a GitHub-hosted authorization job
  and then targets the repository's self-hosted Linux runner. It checks that the
  runner name is `addons` before invoking Codex. Never run fork or pull request
  events on the credentialed runner.
- Use a dedicated Linux machine/VM and non-root service account with its own
  home. No Hub volumes, unrelated repositories, Docker socket, sudo access,
  unrelated credentials, global agent instructions, skills, plugins, MCP
  servers, or hooks. Review machine-managed Codex configuration too.
- Protect the default branch, workflows, and authorization module with maintainer
  review. A user who can change the workflow can bypass its own permission check.
  The module accepts only GitHub `maintain` and `admin` roles; it rejects `write`
  even though the legacy `permission` field also reports `write` for maintainers.
  The original actor and person rerunning a job are both checked live.
- Keep account authentication on the runner's protected disk. Never put it in
  GitHub secrets, repository variables, artifacts, caches, job logs, Hub storage,
  or the checkout. Retain the refreshed auth file between serial runs.
- The probe accepts no user prompt or source context. It inherits only the
  service account home, a fixed system PATH, and locale. It suppresses CLI raw
  output and reports only a validated version and fixed readiness messages.

## Prepare `addons`

1. In this repository's **Settings → Actions → Runners**, verify that `addons`
   is online and registered only here. The job uses the default `self-hosted`
   and `linux` labels and checks `RUNNER_NAME=addons` before the probe.
2. Use Python 3.11+ and the reviewed Codex installation. The probe expects a
   native executable or symlink at `~/.local/lib/hub-agent-runtime/codex`; avoid
   a Node shim when Node is absent from the probe's fixed system PATH. On
   `addons`, the verified native executable is:

   ```text
   /home/r59q/.nvm/versions/node/v26.10.0/lib/node_modules/@openai/codex/node_modules/@openai/codex-linux-x64/vendor/x86_64-unknown-linux-musl/bin/codex
   ```

   Codex 0.158.0 was verified on `addons`. Pin the installed CLI version;
   upgrades require reviewing the symlink target and repeating the diagnostic.
3. Sign in interactively in a private terminal **as the Actions service
   account**, with its normal home and no `CODEX_HOME` override. Do not run login
   inside Actions. Enable device login in account/workspace settings if needed:

   ```sh
   umask 077
   "$HOME/.local/lib/hub-agent-runtime/codex" -c 'cli_auth_credentials_store="file"' login --device-auth
   chmod 700 ~/.codex
   chmod 600 ~/.codex/auth.json
   ```

   Complete the one-time browser approval privately. The file store is selected
   so the service can reuse and refresh its own login without a desktop keyring.
   Do not copy a personal workstation's entire Codex home to the runner.
4. From an operator-reviewed copy of this repository, install the probe outside
   any Actions workspace under the service account. This is already installed
   on `addons`; for replacement or update:

   ```sh
   install -d -m 0700 "$HOME/.local/lib/hub-agent-runtime"
   install -m 0600 ops/private-runner/diagnose.py "$HOME/.local/lib/hub-agent-runtime/diagnose.py"
   # On a new host, create a codex symlink here to its reviewed native executable.
   sha256sum ops/private-runner/diagnose.py
   ```

   Compare the digest with the literal in the diagnostic workflow. The runner
   account owns this installation under the temporary isolation exception.
   Test changes locally, reinstall the reviewed probe, and update the workflow
   digest together whenever the probe changes.
5. Review and place `.github/workflows/agent-runtime-diagnostic.yml` and
   `ops/private-runner/authorize.mjs` on the default branch. There are no
   per-actor variables to maintain. A GitHub-hosted job asks GitHub for the
   current role of both the original and rerun actors; `admin` or `maintain`
   passes. API failures and other roles fail before the self-hosted job starts.
   The workflow runs on manual dispatch, or a default-branch push that changes
   the diagnostic workflow, authorization module, or probe.

## Run and review the spike

From **Actions → Agent runtime diagnostic → Run workflow**, choose the default
branch and dispatch as a repository maintainer. This creates an Actions run
and consumes a small amount of subscription usage for a fixed greeting; it
does not change repository contents, issues, or PRs.

The job checks the runner name and installed probe digest, checks cached ChatGPT
login, then makes one live request in a temporary empty directory with
read-only sandboxing, no approvals, disabled shell/web tools, ignored user
configuration/rules, and ephemeral session storage. API-key/access-token
environment variables are not passed to Codex. Only a successful live reply
produces **Runtime verified** in the job summary. Cached login alone does not
prove account access, quota, or connectivity. A skipped, queued, cancelled, or
failed job is never runtime verification. This does not verify patch generation
or draft-PR authority; those belong to later tasks.

Record these credential-free facts in the task's completion note after review:

- repository and runner scope, actor role check, reviewer, and date;
- Actions run URL and workflow commit;
- runner name, labels, and reported Codex version;
- successful live result and review of this setup/recovery procedure.

The 2026-09-28 run above provides the evidence for AW-001. This guide was
reviewed against the installed runner and completed workflow for setup,
rotation, revocation, and failure recovery. The separate **Agent runtime
checks** workflow runs offline tests on GitHub-hosted runners; it requires no
Codex installation or login.

## Rotation, revocation, and recovery

| Situation | Operator action |
| --- | --- |
| Skipped job | Check default branch, fork status, and authorization result. The workflow does not accept pull request events. |
| Authorization fails | Confirm both original and rerun actors currently have GitHub `maintain` or `admin` roles. Inspect the hosted job if GitHub's permission API is unavailable. |
| Job stays queued | Check `addons` is online and has `self-hosted` and `linux` labels; verify repository/group access. Cancel stale runs before retrying. |
| Probe digest or runner check fails | Review runner labels and operator installation. Reinstall the reviewed probe; never disable checksum verification to get a green run. |
| Version check fails | Restore the reviewed CLI executable. An incompatible CLI must fail visibly, not switch authentication methods. |
| Login not ready | Stop scheduling jobs. In a private service-account terminal, sign out and repeat device login; verify home and file permissions. |
| Live request fails or times out | Inspect account access, subscription usage, connectivity, and CLI compatibility locally. Raw provider errors are intentionally absent from Actions logs. Retry after repair. |
| Cancellation/runner loss | Treat readiness as unverified, cancel pending work, check for leftover Codex processes/temp directories locally, and rerun after recovery. |
| Planned rotation | Drain/stop the runner; run `codex -c 'cli_auth_credentials_store="file"' logout` as its service account, sign in again privately, secure file permissions, restart and rerun the diagnostic. |
| Suspected compromise | Stop/remove the runner and disable the diagnostic workflow immediately. Revoke the account's sessions/access through account/workspace controls; local logout alone is not proof of remote revocation. Rebuild the runner from a clean image and authenticate afresh. |
| Runner replacement | Remove old registration and credentials, securely retire its disk, provision a clean dedicated host, repeat setup and record a new run. Do not restore a whole runner home/workspace backup. |

## Development and tests

Runtime uses only Python's standard library. Tests use a disposable fake Codex
executable and PyYAML; Node tests GitHub role handling with a fake API; Ruff
formats/lints Python and actionlint validates Actions.
Install pinned development tools once (Go is also needed for actionlint):

```sh
python3 -m venv ops/private-runner/.venv
ops/private-runner/.venv/bin/python -m pip install -r ops/agent-profiles/requirements-dev.txt
make runtime-check
```

The combined requirements also install the
[profile validator](../agent-profiles/README.md); `make check` includes both
components' checks. To format a probe change, run
`ops/private-runner/.venv/bin/python -m ruff format ops/private-runner`, then
update its workflow SHA-256. `RUNTIME_PYTHON` can point to another environment
containing the pinned tools. No test contacts GitHub or OpenAI. The tests cover
safe success/failure output, credential environment isolation, early auth
failure, timeout, cleanup, maintainer role handling, and the workflow guard.

## References reviewed

- [OpenAI authentication](https://learn.chatgpt.com/docs/auth) describes device
  login and file/keyring credential storage.
- [Advanced account authentication in CI](https://learn.chatgpt.com/docs/auth/ci-cd-auth)
  describes the sensitive persisted-account pattern selected for this spike.
- [Codex command reference](https://learn.chatgpt.com/docs/developer-commands?surface=cli)
  documents non-interactive execution and configuration isolation.
- [GitHub runner labels](https://docs.github.com/en/actions/how-tos/manage-runners/self-hosted-runners/apply-labels)
  and [runner security](https://docs.github.com/en/actions/reference/security/secure-use)
  explain scheduling and the risks of credentialed self-hosted runners.
- [GitHub collaborator permissions](https://docs.github.com/en/rest/collaborators/collaborators#get-repository-permissions-for-a-user)
  defines the `role_name` response used by the hosted authorization job.

## Profile readiness (AW-006)

The legacy `addons` probe above remains unchanged and cannot verify
`codex-thorough`, `hub-agent-codex`, or an exact model policy. The new fixed,
private-repository-only producer and versioned evidence contract are described
in [READINESS.md](READINESS.md). Its offline tests are part of `make runtime-check`.
Dedicated runner provisioning and live verification remain AW-012.
