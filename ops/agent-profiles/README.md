# Repository agent profile contract (AW-003)

This component defines the Agents domain's repository-local profile format and
the v1 boundary between a profile and a GitHub Actions adapter. It provides an
offline validator, not a service or execution runtime. It reads no GitHub data,
invokes no Codex process, and stores no profile or authentication state.
The Agents service consumes this schema through a generated embedded snapshot;
its domain models remain independent of transport fields. The server-only frontend
adapter validates profiles with the same schema.

## Catalog and versioning

The only catalog is [`.github/agent-profiles.yml`](../../.github/agent-profiles.yml).
[The JSON Schema](schema.v1.json) is the normative v1 field contract, using
Draft 2020-12. Every object is closed except the profile-ID map. Fields are
required explicitly; there are no hidden defaults, environment substitutions,
credential fields, command strings, executable paths, custom provider URLs,
workflow references, or arbitrary CLI arguments.

Catalogs use one UTF-8 YAML document, at most 65,536 bytes, at most 24 nesting
levels, and 1–32 profiles. Use string mapping keys and lowercase `true`/`false`
booleans. Duplicate keys, explicit tags, anchors, aliases, and merge keys are
rejected. Profile IDs are lowercase kebab-case, at most 64 characters.
Catalog prose is public configuration: never put credentials or private source
content in any field, even a display name or description. Display prose is not
an executable instruction or authority grant.

`schema_version: 1` versions the catalog. `adapter.contract_version: 1` versions
the invocation/result contract below. Neither is a profile revision. AW-002
assignments pin the full 40-character Git commit SHA containing the catalog:

```text
/agent assign codex-thorough@0123456789abcdef0123456789abcdef01234567 authority=branch-draft-pr
```

The GitHub-hosted intake must verify that this SHA is in the current default
branch's history and read the catalog at exactly that SHA. No mutable branch,
tag, shortened SHA, external catalog, or profile-local `revision` field is
accepted. The schema and adapter implementation are trusted, reviewed intake
code; an assignment cannot select alternate validator or workflow code.

## Profile fields

| Field | V1 meaning |
| --- | --- |
| `enabled` | Both the pinned and current entries must be enabled. Setting the current entry to `false`, removing it, or making the current catalog invalid blocks intake and continuation. |
| `name`, `description`, `role` | Display identity and `implementation` role; not prompt or shell content. |
| `adapter` | `codex-chatgpt-private-runner`, contract v1, and a dedicated custom runner label. Built-in labels alone are invalid. The adapter adds `self-hosted` and `linux`. Labels select a runner; repository registration or runner-group restrictions authorize repository access. |
| `model` | Exact model ID and requested reasoning effort (`low`, `medium`, `high`, `xhigh`, or `max`); `fallback: none`. No provider or account substitution. Availability is a separate runtime check. |
| `triggers` | Fresh issue assignment through `issue_comment.created`. Edited comments and fresh PR assignments are unsupported in v1. |
| `context.sources` | Allowed live sources. `issue` and `repository` are mandatory. Optional `issue_comments`, `instructions`, `pull_request`, `review_thread`, and `checks` enable collection appropriate to the invocation. |
| `context.images` | `false`; image retrieval and visual workflows are deferred. |
| `authority` | `branch-draft-pr`, `workspace-write` sandbox, and `network: false`. The network restriction applies to workload tools, not the adapter's GitHub collection or model transport. |
| `validation` | Named check IDs, initially `repository-check`, with `on_failure: draft-with-evidence`. A failing check must remain visible on a draft, never imply validated success. |
| `continuation` | `review_comments` permits trusted review-thread follow-up on the verified owning PR. When true, PR, review-thread, and checks context must be enabled. `pipeline: disabled` excludes event-driven remediation. |

The catalog requests `gpt-6.1-sol` with `high` reasoning and no fallback. This is
a configuration choice, not a statement that the existing runner/account has
verified it. The adapter must verify compatibility with its pinned CLI and
account and report `MODEL_UNAVAILABLE` or `MODEL_POLICY_UNSUPPORTED` safely.
Official [model documentation](https://learn.chatgpt.com/docs/models) and
[configuration reference](https://learn.chatgpt.com/docs/config-file/config-reference)
were reviewed on 2026-09-30. `model.id` maps to explicit Codex model selection;
`reasoning_effort` maps to `model_reasoning_effort`. The adapter must confirm the
effective policy and fail on any unsupported or substituted setting. Ultra,
other adapters, API-key authentication, image workflows, and pipeline
remediation are outside v1.

The `repository-check` ID maps to the reviewed adapter's fixed `make check`
invocation in the isolated repository checkout. The catalog cannot redefine
it. Repository check recipes are workload code and run within the same sandbox
and credential restrictions as the agent. Missing tooling/entry point is
`VALIDATION_UNAVAILABLE`; it is recorded as unavailable, not passed. There is
no automatic dependency installation or unsandboxed retry. A new check ID
requires a reviewed adapter/schema change.

## Adapter contract v1

These are the logical inputs and outputs for AW-011–AW-013, rather than an HTTP
API. The [AW-011 intake](../agent-intake/README.md) implements assignment input
verification and its invocation artifact; AW-012/AW-013 provide execution/results.
GitHub Actions transports verified metadata as structured data, never interpolated
shell commands.

### Input and authorization

The hosted intake produces an invocation with `contract_version: 1`, an
`assignment_id` derived from the repository ID and immutable request comment
ID, and `operation: assignment` or `review-continuation`. It supplies:

| Input | Verified origin |
| --- | --- |
| Repository | GitHub numeric repository ID, owner/name, approved scope, and default branch. |
| Source and request | Issue number, immutable assignment comment ID/URL, and current open source/comment state. A continuation also supplies PR number and review comment ID. |
| Identity | Current requester's GitHub account ID/login and live `maintain`/`admin` role; the rerun actor is checked independently. |
| Profile | Exact profile ID, pinned `profile_revision` SHA, validated pinned profile, and independently validated current default-branch entry at `policy_revision`. |
| Checkout | Full `base_sha` resolved from the current default branch for assignment, or verified owning PR's current `head_sha` for continuation; never a model-provided ref. |
| Run | GitHub workflow run ID, attempt, and URL for receipts and provenance. |

Reject malformed assignments, deleted/edited comments, closed sources,
unapproved repositories, fresh PR assignments, and untrusted actors before
scheduling the credentialed runner. Repeated delivery/rerun of the same request
must reuse its assignment identity and cannot create another branch/PR.
Follow-up uses verified AW-002 provenance, checks the linked request/run/branch,
and accepts only a trusted new review comment on that assignment's open PR.
A PR body alone cannot authorize execution.

Both catalogs must validate. Both entries must exist and be enabled. Compare
every execution field (all fields except `name`, `description`, and `enabled`)
against current policy, treating context/check lists as sets. Any difference
requires a new assignment and produces `PROFILE_POLICY_CHANGED`; do not
silently combine revisions. This conservative rule also covers runner, context,
validation, and continuation changes. A current revocation cannot be undone
by pinning an old catalog. Revalidate current policy and authority before every
continuation and immediately before the write job applies a patch.

### Collection and execution

The adapter collects current authorized GitHub context, filtered by
`context.sources`. An assignment uses issue context; a continuation uses the
latest owning PR head/diff, review thread, and checks. No context field
authorizes reading unrelated repositories, credentials, attachments, or host
files. Issues, comments, instructions, diffs, and check evidence are untrusted
workload input. Do not evaluate them as shell code or use them to alter
adapter configuration, tool policy, model selection, or write authority.

The credentialed job has repository-read GitHub access, no GitHub write token,
an isolated checkout, and the minimum protected account authentication. It
produces a patch rather than pushing. The adapter enforces filesystem/tool
isolation, denies workload network access and access to host authentication,
ignores ambient/repository Codex configuration, and exposes only explicitly
permitted repository instructions. CLI sandbox selection alone is not proof
that host credentials are inaccessible. Do not inherit unrelated environment
variables, plugins, MCP servers, hooks, or user rules. Provider authentication
remains in the operator's protected runtime boundary, never in invocation
metadata, checkout, artifacts, caches, results, or logs.

Use a dedicated restricted runner host/account before enabling source execution.
The shared `addons` exception in AW-001 authorized the fixed diagnostic only;
this catalog does not extend it. The custom `hub-agent-codex` label and model
policy still require setup and verification. The intake is implemented, while
the dedicated patch job remains gated until AW-012. Execution is unavailable
even when this catalog validates.

### Output and separate write job

Every attempt produces a bounded structured result containing:

| Output | Meaning |
| --- | --- |
| Identity | `contract_version`, `assignment_id`, `operation`, `profile_id`, `profile_revision`, `policy_revision`, workflow run ID/attempt, and exact `base_sha`/`head_sha`. |
| Outcome | `ready`, `blocked`, `failed`, or `cancelled`. `ready` means a proposal can be published as a draft, not that all checks passed. |
| Policy evidence | Requested and verified effective model ID/reasoning effort, and applied authority/sandbox policy. |
| Validation | One entry per requested check: ID, `passed`/`failed`/`unavailable`/`cancelled`, and bounded safe evidence. Never publish raw provider output, environment dumps, or private source content in public logs. |
| Proposal | A bounded patch artifact reference/digest and summary artifact, only for a `ready` result; never executable code/commands in metadata. No changes produce `blocked` with `NO_CHANGES`. |
| Recovery | Stable safe reason code/message and next action for blocked/failed/cancelled results. Cancellation/runner loss may require the hosted reporting job to synthesize this result. |

Adapter failures use fixed codes such as `AUTHORIZATION_DENIED`,
`PROFILE_DISABLED`, `PROFILE_POLICY_CHANGED`, `RUNNER_NOT_READY`,
`MODEL_UNAVAILABLE`, `MODEL_POLICY_UNSUPPORTED`, `VALIDATION_UNAVAILABLE`,
`NO_CHANGES`, `PROTECTED_CHANGE`, `STALE_HEAD`, `RUNTIME_FAILED`, and
`CANCELLED`. Raw API/CLI exceptions cannot be forwarded to GitHub.
Patch/summary artifacts can contain repository material: keep them under the
repository's existing access controls and retention policy. For a public
repository, collected context and artifact contents must be public-safe.

Only the separate write job has branch/PR/check/comment authority, with no
model credential. It verifies the artifact's run/assignment identity and digest,
current policy, expected source/head state, and protected paths before applying
it. Never trust the model's result metadata as authorization. Reject changes to
`.github/**`, `ops/private-runner/**`, `ops/agent-profiles/**`, any `AGENTS.md`,
`.codex`/`.agents`/`.git` directory, or `.env`/`.env.*` files. Reject path escapes,
symlinks outside the checkout, and submodule changes. Profiles cannot relax
these adapter-owned deny rules. Other reviewed protected paths may be added
by the adapter; they require a fresh assignment if effective policy changes.

Create one dedicated branch attributable to the assignment and a draft PR with
AW-002 provenance, validation evidence, and links to the request/run. A
continuation can update only that verified owning branch. Never merge, release,
modify secrets/permissions, or update another branch. A stale continuation head
produces `STALE_HEAD` and requires fresh context; do not force-push. Publish safe
summary/check evidence for success and recovery guidance for every failure.

## Offline validation and development

Python 3.11+ is required. Dependencies are pinned in
[`requirements-dev.txt`](requirements-dev.txt), reusing the existing Python
Ruff/PyYAML versions and Ruff rules. `jsonschema` performs schema validation;
the SafeLoader adapter only enforces the bounded YAML subset. There is no
custom schema interpreter or general YAML parser.

From the repository root, install the combined tools in the existing environment:

```sh
python3 -m venv ops/private-runner/.venv
ops/private-runner/.venv/bin/python -m pip install -r ops/agent-profiles/requirements-dev.txt
make profiles-check
make check
```

`PROFILE_PYTHON` can select another environment with the pinned tools. The
validator also accepts a local catalog path:

```sh
ops/private-runner/.venv/bin/python ops/agent-profiles/validate.py
```

It exits `0` for valid configuration or `1` for validation/file errors and emits
only JSON `valid` plus `errors`. Each error has `code`, `path`, and a fixed
actionable `message`. Errors sort by path/code/message and are deduplicated.
Paths redact profile IDs as `<profile>` and unknown keys as `<field>`; neither
source values nor parser/library exception messages are echoed. Catalog/file
errors use `/`. Examples include `MISSING_FILE`, `UNREADABLE_FILE`,
`INVALID_YAML`, `DUPLICATE_KEY`, `UNSUPPORTED_VERSION`, `MISSING_FIELD`,
`UNSUPPORTED_FIELD`, `UNSUPPORTED_VALUE`, `INVALID_TYPE`, `INVALID_FORMAT`,
`DUPLICATE_VALUE`, `MISSING_CONTEXT`, and `LIMIT_EXCEEDED`.

The GitHub-hosted offline workflow and `make check` validate the committed
catalog/schema, meaningful rejection behavior, safe CLI output, and Python,
JSON, and YAML formatting. No test uses GitHub, OpenAI, runner authentication,
or a self-hosted runner. These tools live outside Compose because they are
repository-contract tooling, not application services.

## Service contract exports (AW-005)

`export_contract.py` is the build-time bridge to the Agents read API. It copies
this normative schema into checked-in Go/frontend snapshots and derives the
`Catalog*` OpenAPI transport field shapes. OpenAPI 3.0 lacks the original
conditional/contains semantics; the transport projection omits these and regex
patterns, while Go and the server-only frontend validate with the full original
Draft 2020-12 schema. No consumer has a separately maintained field policy.

Run `make generate-agents-contract` after schema/API changes; do not edit the
generated snapshots or `Catalog*` definitions. `make agents-contract-check`
verifies these exports and generated Go/TypeScript DTOs without editing files.
The service needs no Python runtime, and generation needs no GitHub credentials.

The exporter also consumes the separate [runtime evidence schema](../private-runner/readiness.schema.v1.json)
to generate `RuntimeEvidence` transport shapes and Go/server-only frontend
snapshots. `make agents-contract-check` verifies both contracts without editing
files. Profile configuration validation and runtime evidence validation have
separate semantics; see the [readiness guide](../private-runner/READINESS.md).

AW-014 also exports the canonical [patch-result schema](../private-runner/result.schema.v1.json)
as a Go-only embedded snapshot for read-side assignment evidence validation.
It does not copy result policies into frontend code or change the v1 adapter.
`make generate-agents-contract` rebuilds it; `make agents-contract-check`
verifies that its bytes still match the canonical source.
