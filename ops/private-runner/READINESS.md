# Profile readiness evidence v1 (AW-006)

This component publishes safe evidence for the Agents readiness reader. It does
not execute source, accept issue prompts, provision a runner, or authorize an
assignment. The legacy AW-001 `addons` diagnostic remains separate.

## Fixed producer and operator setup

`.github/workflows/agent-profile-diagnostic.yml` is manual-only, default-branch,
non-fork, and private-repository-only by default. The operator explicitly
approved shared `addons` for public `r59q/hub-rearranger` on 2026-10-02; this
exact named exception retains all role/default-branch/no-fork guards. Its hosted `authorize` job checks both
original and rerun actors' current `maintain`/`admin` permission and validates the
commit-pinned catalog against the fixed probe policy. The credentialed
`diagnostic` job requests `[self-hosted, linux, hub-agent-codex]`, has no GitHub
token permissions, runs no checkout/actions, and accepts no prompt, runner,
model, profile, or path inputs. It verifies the reviewed scripts' checksums before
executing the operator-installed probe. The hosted `publish` job uploads only
the safe record with pinned `actions/upload-artifact` v4, one-day retention, and
an immutable name `agent-readiness-v1-{run_attempt}`.

Provision the dedicated runner under AW-012 using the base account/installation
practices in [README.md](README.md), **not the shared `addons` runner**. Keep its
service-account home and installed files operator-controlled. Install reviewed
`diagnose.py` and `profile_diagnostic.py` together at
`~/.local/lib/hub-agent-runtime/`, alongside the pinned `codex` binary. The new
probe reuses the legacy script's credential-stripping subprocess adapter; both
script digests are pinned in the new workflow. No authentication file is copied
or uploaded. After a probe update, run Ruff, update its SHA-256 in the workflow,
review the scripts, and reinstall them on the runner before dispatching.

The fixed probe requests `codex-thorough`, `gpt-6.1-sol`, `high`, and
`hub-agent-codex`. Other profiles/policies remain pending until a reviewed fixed
producer supports them. It uses an empty disposable directory, read-only sandbox,
no user config/rules, no shell tools/search, an ephemeral session, and a constant
one-line prompt. Only cached ChatGPT authentication is permitted. Version and
login output are checked privately; a successful live request must return the
fixed marker and the CLI session header must identify the exact model and
reasoning effort. Missing, ambiguous, adjusted, or unsupported headers fail
closed; requested arguments alone never count as effective-policy proof.

The header is CLI session-policy evidence, not independent provider telemetry.
A reroute or warning prevents verification. Operator-managed policy can still
restrict model access; repair locally rather than changing model/account as a
fallback. [Official configuration documentation](https://learn.chatgpt.com/docs/config-file/config-advanced)
describes `--model` and reasoning overrides. The [CLI human-output implementation](https://github.com/openai/codex/blob/main/codex-rs/exec/src/event_processor_with_human_output.rs)
provides the session-model header and reroute reporting checked by this probe.
The complete source-execution isolation boundary remains AW-012/AW-013.

## Contract and reader trust

[`readiness.schema.v1.json`](readiness.schema.v1.json) is the normative closed
Draft 2020-12 schema. A record contains:

- `version`, repository, profile ID/full commit revision, and expected custom runner label;
- requested/effective model and reasoning effort, plus validated CLI version;
- workflow run ID/attempt, UTC verification time, and fixed outcome/reason code.

No authentication identity, access token, auth file, prompt transcript, source,
stdout/stderr, exception, arbitrary message, or provider error is allowed.
Failed records may leave effective policy/CLI fields null. A verified record
requires all of them and reason `verified`. Producer process failures return a
safe failed record; failed authorization, wrong script digest, cancellation,
missing runner, upload failure, or malformed records remain unverified/pending.
The diagnostic job exits successfully after emitting either valid outcome;
its green job status alone never establishes runtime success.

The reader only considers the latest run of the registered
`.github/workflows/agent-profile-diagnostic.yml` on the current default branch.
It checks the registered workflow ID, run path/event/repository/head repository,
full head SHA, and current attempt. It reads exact-attempt job metadata:
`authorize` and `diagnostic` must each be unique, completed successfully, and
bound to that run/attempt/SHA; the diagnostic's scheduler labels must include
`self-hosted`, `linux`, and the profile label. Trust comes from reviewed workflow
code on the repository's default branch and GitHub job metadata, not claims
inside an artifact. Repository maintainers control that workflow source.

Artifact metadata must bind to that run, repository, branch, and SHA. Its
immutable attempt name and SHA-256 digest must match. The reader downloads with
a separate unauthenticated HTTPS client, disallows redirects, bounds compressed
and uncompressed data, requires exactly one regular `readiness.json`, and
validates the complete canonical schema before returning any data. The shared
contract exporter keeps embedded Go/frontend schemas and OpenAPI shapes aligned.

Policy matching requires the record's repository/profile/revision/runner and
requested model/effort to equal the current catalog. Verified effective policy
must equal the request. The run's head must equal the catalog's current full
commit SHA. Evidence expires after **24 hours**, including failed evidence;
one-minute future-clock tolerance is allowed, and verification time must fall
within diagnostic job timestamps. A new commit or latest run/attempt invalidates
older proof. There is no fallback to an older successful run when the newest is
queued or lacks valid evidence. Legacy workflow results and `addons` labels
cannot verify a dedicated profile.

## Safe outcomes and recovery

| Reason                         | Local action before rerunning                                                               |
| ------------------------------ | ------------------------------------------------------------------------------------------- |
| `verified`                     | Refresh after repository changes or within 24 hours.                                        |
| `installation_unavailable`     | Restore the pinned operator-installed CLI and probe scripts.                                |
| `authentication_unavailable`   | Repair cached ChatGPT sign-in privately using the base operator guide.                      |
| `request_failed`               | Inspect local installation, connectivity, subscription access/quota, and CLI compatibility. |
| `effective_policy_unavailable` | Check CLI header compatibility and exact model/effort access; do not substitute a fallback. |

The Agents API also names missing bootstrap files and workflow metadata.
Configuration presence does not prove execution isolation or authorize work.
Bootstrap templates remain AW-008; UI presentation remains AW-007. This task's
producer and reader are validated offline, without dispatching a credentialed
workflow or claiming that the future dedicated runner is provisioned.

## Offline development

Install the combined tools from [the base README](README.md#development-and-tests).
Run `make runtime-check`, `make generate-agents-contract` after schema/API changes,
and `make check` before handoff. Fake process tests verify explicit policy,
credential isolation, cleanup, effective-header failures, safe schema outcomes,
metadata validation before authentication, and workflow guards/checksums. Go
uses controlled REST/HTTPS servers to test origin, attempts, artifact limits/
digest/expiry, permissions, and token-free downloads; no live account is needed.
