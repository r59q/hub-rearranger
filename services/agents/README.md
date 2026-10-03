# Agents service (AW-004–AW-014)

The Agents service owns repository profiles, their validation, derived readiness,
the GitHub-native assignment convention, and GitHub-derived assignment activity. It has no queue, runtime, provider
credential store, transcript store, or durable assignment/profile database.
GitHub remains the source of truth. Runners and their authentication live outside
Hub and Docker Compose. Readiness derives configuration and trusted diagnostic evidence; profile validation alone
never proves that a runner, model, or account is ready.

## Local setup and configuration

Use Go 1.25. Public catalogs can be read without a token; private repositories
require a fine-grained token with read-only Metadata and Contents access. Readiness also requires Actions read access. Assignment views use read-only Issues,
Pull requests and Checks permissions alongside Metadata, Contents and Actions:

```sh
GITHUB_TOKEN=github_pat_... go run ./cmd/server
```

`GITHUB_TOKEN` stays in the Go process. No provider login, database, or runner
is needed. `AGENTS_ADDR` defaults to `127.0.0.1:8082`; Compose uses `:8082` on
its private network, publishes the port on loopback, and passes the root read
token. `AGENTS_PORT` overrides the published port. Environment files are
ignored and never loaded automatically by the service.

The API has no caller authentication and projects repositories visible to its
server-side token. Keep it on loopback or a private application network and
access it through SvelteKit's server adapter. Do not publish it as a public
service. A future multi-user deployment needs caller authorization before
exposing private repository data.

## Layers and integration points

- `cmd/server` composes dependencies, HTTP timeouts, logging, and graceful shutdown.
- `internal/api` validates transport input and maps domain results into generated DTOs.
- `internal/domain` owns convention, profile-read, and readiness use cases through reader/validator interfaces.
- `internal/infrastructure/github` reads GitHub through pinned `go-github`.
- `internal/infrastructure/profiles` implements bounded YAML parsing and canonical JSON Schema validation.
- `internal/infrastructure/evidence` validates the closed runtime evidence schema.
- `internal/infrastructure/config` reads server environment settings.
- `api/openapi.yaml` is the versioned public API contract.
- `frontend/src/lib/server/agents-api.ts` is the typed server-only consumer.

## Public API

[`api/openapi.yaml`](api/openapi.yaml), version 1.4.0, uses OpenAPI 3.0.3.
The API URL version and catalog schema version are separate contracts.

- `GET /health` returns `{"status":"ok"}` independently of GitHub and runners.
- `GET /v1/assignment-convention` describes convention v1, catalog schema v1,
  issue-comment assignment, required current `maintain`/`admin` roles,
  `branch-draft-pr` authority, and a full default-branch ancestor SHA.
- `GET /v1/repositories/{owner}/{repo}/profiles` reads and validates
  `.github/agent-profiles.yml` in the requested repository.
- `GET /v1/repositories/{owner}/{repo}/readiness` derives per-profile configuration/runtime state.
- `HEAD` on these endpoints checks read availability without a response body.
  A `200` profile response may still describe an invalid or missing catalog;
  use GET to inspect its state.

Each profile read fetches repository metadata, resolves the default branch to
a full commit SHA, and reads the file at that exact SHA. The response includes
`repository`, `catalog_path`, `default_branch`, `revision`, `state`, `profiles`,
and `diagnostics`. Each valid profile has `id`, the same commit `revision`, and
a typed `configuration` containing all v1 policy fields. Revisions are Git
commit SHAs, never the Contents API blob SHA. They identify the exact catalog
snapshot displayed; this endpoint does not authorize a later assignment or
perform pinned/current execution-policy checks.

`state` is `valid`, `missing`, `invalid`, or `unsupported` (unknown catalog
schema version). Missing files, malformed YAML, unknown fields/values, unsupported
file types, and size/depth limits have safe actionable diagnostics. Invalid
catalogs return no partially parsed profiles. Disabled profiles remain visible
when the catalog validates. Profiles sort by ID; diagnostics sort and deduplicate
by path/code/message. Profile IDs and unknown keys are redacted in error paths;
raw YAML, parser exceptions, GitHub errors, and repository content never enter
logs. Display text in valid profiles is repository-owned prose, never executable
instructions; repositories must not put credentials in catalog fields.

Repository access failures return `403` or `404`, rate limits `429`, GitHub
failures `502`, invalid owner/name `400`, and internal failures `500`, with
safe `Error` responses and recovery guidance. Unsupported methods return `405`
with `Allow: GET, HEAD`; unknown paths return `404`. All reads use `no-store`.
Every request fetches fresh GitHub data; there is no profile persistence or cache.
Requests have an eight-second upstream deadline and propagate cancellation.

## Development, contracts, and tests

Use the repository-wide tool versions and commands:

```sh
gofmt -w cmd internal
go vet ./...
go test ./...
```

After frontend and Python development-tool setup, from the repository root:

```sh
make generate-agents-contract
make agents-contract-check
make check
```

[`ops/agent-profiles/schema.v1.json`](../../ops/agent-profiles/schema.v1.json)
is the normative schema. `export_contract.py` exports its checked-in Go and
frontend schema snapshots and the `Catalog*` OpenAPI field shapes; do not edit
these generated definitions. The transport projection converts constants to
enums and omits JSON Schema conditionals and regex patterns that OpenAPI 3.0
cannot faithfully validate. Full semantics are checked with the original
Draft 2020-12 schema by `jsonschema/v6` in Go and Ajv in the server-only frontend
adapter. Go YAML parsing preserves the AW-003 size/depth/ambiguity restrictions;
`regexp2` supports the canonical runner-label negative lookahead. The Go service
needs no Python runtime. Python is only used during contract generation/checks.

Pinned `oapi-codegen` generates Go DTOs/routes, and `openapi-typescript` generates
frontend types and enum values. Domain models remain independent of transport
DTOs. Contract checks verify schema snapshots and transport projection, then
compare regenerated Go/TypeScript code without changing files.

Domain tests cover fresh reads, revision propagation, ordering, validation
states, upstream failures, and cancellation. Controlled GitHub HTTP tests cover
commit pinning, file absence/types/limits, credentials, and access/rate errors.
API integration tests exercise valid `codex-thorough`, invalid schema/YAML,
unsupported versions, missing catalogs, safe failures, and HEAD against OpenAPI.
Frontend tests cover typed reads, schema validation, revision consistency,
malformed responses, and useful transport errors. No test needs live GitHub,
OpenAI, runner authentication, or a self-hosted runner.

## Readiness derivation

Readiness reads the catalog and required files at the same default-branch commit,
then projects current Actions workflow/run/job/artifact metadata. It returns
`catalog_state`, catalog diagnostics, and sorted profile rows with `state`,
`runner_label`, `next_action`, safe diagnostics, and optional matching evidence.
Invalid/missing catalogs have no profile rows. Disabled profiles remain visible;
readiness never grants assignment authority. Requests have a 20-second deadline.

Required regular, nonempty files (each at most 64 KiB) are `AGENTS.md`,
`docs/agent-workflows.md`, `.github/workflows/agent-assignment.yml`, and
`.github/workflows/agent-profile-diagnostic.yml`. Workflow YAML uses the catalog's
bounded, unambiguous parser. Both workflows must be registered and active in
Actions. Assignment metadata needs `issue_comment` with `created` and jobs named
`authorize`, `patch`, and `publish`; diagnostic metadata needs manual dispatch
only and jobs named `authorize` and `diagnostic`. The patch/diagnostic jobs must
statically request `self-hosted`, `linux`, and the profile's custom runner label.
Dynamic runner expressions cannot establish readiness. These presence/metadata
checks do not audit workflow security or prove execution isolation (AW-012/AW-013).
AW-008 owns complete bootstrap templates; this task installs no assignment stub.

| State                   | Meaning and next step                                                                                                                                                    |
| ----------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `configuration_missing` | Add named missing files or repair workflow registration/metadata.                                                                                                        |
| `verification_pending`  | Run the dedicated profile diagnostic on a private repository's default branch. Evidence is absent, stale, malformed, or mismatched, or the newest attempt is incomplete. |
| `runtime_verified`      | Fresh matching no-write diagnostic succeeded; refresh after changes and within 24 hours.                                                                                 |
| `verification_failed`   | A fresh matching diagnostic returned a safe failure; repair the runner locally and rerun.                                                                                |

The [evidence contract and operator guide](../../ops/private-runner/READINESS.md)
define origin, freshness, and exact policy matching. Only the latest run on the
default branch is considered, using exact-attempt jobs and immutable attempt
artifact names. A newer queued run never falls back to an older success.
The run's head commit must equal the current catalog revision, so _any_ new commit
requires a new diagnostic. Records are valid for 24 hours inclusive, with at
most one minute future-clock tolerance, and must fall within the diagnostic
job's timestamps. Matching failed records expire under the same rules.

GitHub access/rate failures remain transport errors with recovery guidance;
they are not reported as missing configuration. Missing/expired/malformed or
unverifiable artifacts remain pending. Downloads use go-github's supported
artifact redirect API and a separate HTTPS client without GitHub credentials,
with redirects disabled. Archives are SHA-256 checked, bounded to 64 KiB, contain
exactly one regular `readiness.json` (16 KiB maximum), and are never extracted.
The canonical evidence schema validates all fields before domain matching.
Raw process output and untrusted artifact strings never reach responses/logs.

The contract exporter also derives `RuntimeEvidence` OpenAPI shapes and Go/
frontend schema snapshots from `ops/private-runner/readiness.schema.v1.json`.
Regenerate/check them with the same Agents contract commands. Offline tests
cover all four states, freshness, model/runner/origin mismatches, rerun behavior,
safe producer failures, artifact digest/shape/expiry, token-free downloads,
Actions access errors, API methods/contracts, and server-only frontend validation.
The profile/readiness UI remains AW-007.

## Bootstrap package generator (AW-008)

The local `cmd/bootstrap` command prepares reviewable files without contacting
GitHub, changing a target checkout, or enabling a runner. From this component:

```sh
go run ./cmd/bootstrap --source ../.. --target /path/to/target --format diff
go run ./cmd/bootstrap --source ../.. --target /path/to/target --output /tmp/agent-bootstrap-package > /tmp/agent-bootstrap-plan.json
```

Omit `--target` to plan an empty repository. `--output` must name a new directory
outside both checkouts whose parent exists. It exports the complete package, including unchanged files,
for review/staging; it cannot overwrite an existing directory. Apply reviewed
changes separately. Default JSON output includes version 1, sorted files,
`create`/`update`/`unchanged`/`conflict` status, SHA-256 and proposed UTF-8 content,
plus safe diagnostics. `--format diff` uses Git to render an applicable patch,
including additions and missing final newlines. No timestamps, temporary paths,
tokens, manifests, virtualenvs or runner authentication enter generated files.
Review output locally when comparing a private checkout.

`internal/domain` owns the pure plan and bounded managed `AGENTS.md` addition.
`internal/infrastructure/bootstrap` reads canonical checkout sources and a
confined target snapshot; the existing schema validator merges the catalog.
A matching `codex-thorough` policy preserves the entire catalog byte-for-byte.
A missing profile is added while retaining other entries/comments (YAML formatting
is normalized only for this insertion). Different or disabled profile policy,
invalid catalogs, or ambiguous instruction markers require manual resolution;
they are never silently overwritten or enabled. Existing instructions outside
the managed block are preserved. Other canonical files are proposed as explicit
diffs, so upgrades remain reviewable. Exit codes are 0 for a valid plan, 1 for
I/O/template/usage failure, and 2 for a policy/instruction conflict. Conflicts
prevent package export. Files and path components must be regular UTF-8 files
without symlinks, and individual source/target files are bounded to 1 MiB.
Proposed readiness files, including the merged instructions, also respect the
reader's 64 KiB limit, so a successful plan remains usable by readiness checks.

The package uses the existing AW-011–AW-013 workflow/helper sources and AW-003 /
AW-006 schemas, not copied execution templates. The only workflow transformation
removes the upstream repository's public-runner exception and replaces its
diagnostic comment with the private installation policy. That exact exception
does not transfer to targets. `AGENTS.md`, `docs/agent-workflows.md` and workflow
paths match the readiness contract. Included source/tests, pinned dependencies,
component docs/ignore rules, and the canonical `Agent bootstrap checks` workflow
make the exported installation independently buildable and verifiable.

Use a reviewed Hub Rearranger source checkout to run the generator; operating the
installed flow requires neither the generator nor Hub. The generator alone adds
no server API or Compose dependencies. AW-009 below adds the separate review and
Identity-authorized PR action. The CLI needs no runtime configuration.
[The installation guide](../../docs/agent-workflows.md) explains manual dedicated
runner provisioning, exact model/isolation verification, fresh origin-verified
diagnostics, both disabled-by-default gates, recovery and revocation.

Go tests cover deterministic plans, catalog preservation/conflicts, instruction
merges, symlink/binary/size rejection, checksum binding, CLI export, idempotence
and applicable Git diffs. `make bootstrap-check` uses the pinned Python tools to
export into a disposable directory, prove an empty repeat diff, and run the
generated installation's own canonical hosted checks without the upstream source
tree or Hub. This includes Go compilation/tests, Python/Node/schema checks and
actionlint. It is included in `make check`; all tests are offline and use no
Codex authentication. This local tool's Python check is development-only.

## Hub bootstrap review (AW-009)

`GET /v1/repositories/{owner}/{repo}/bootstrap` returns a fresh, commit-pinned
canonical plan, original blob IDs, proposed hashes and an applicable Git diff.
`HEAD` checks availability without returning source. The review digest binds the
repository, default branch, base and every original/proposed file; it is context,
not a write grant. Conflicting policy/instructions and already matching setups
have explicit states. No preview, draft or competing repository configuration is
persisted. Missing/uninitialized/archived repositories, unsafe files, truncated
trees and unbound blobs fail closed with safe recovery guidance.

Local development from this module reads the reviewed source checkout at `../..`;
set `AGENTS_BOOTSTRAP_SOURCE` to use another reviewed root. Production uses
`AGENTS_BOOTSTRAP_BUNDLE`: the root-context Docker build runs the AW-008 generator
against the canonical sources and packages its verified JSON output. Rebuild the
image after changing those sources. The runtime includes Git for isolated diff
rendering; it never executes proposal source. The root Docker ignore rules exclude
credentials, local environments and generated work directories from that context.

The server-only frontend adapter maps the generated public contract to review
fields. Identity consumes a separately generated, narrow copy of this same public
contract; Agents receives no user token or session. Identity independently obtains
the fresh preview and authorizes/publishes the reviewed GitHub changes. Discovery's
read-only `GITHUB_TOKEN` can never authorize that publication.

Run `make generate-agents-contract` after contract changes and
`make agents-contract-check` to verify all consumers. Tests exercise digest binding,
container bundle integrity, commit-pinned reads, unsafe snapshots, contract states
and failures. The portable package smoke check remains `make bootstrap-check`.

## Profile authoring (AW-010)

`GET /v1/repositories/{owner}/{repo}/profile-editor` returns current/default
editable choices and the fresh base revision, or safe catalog/policy diagnostics
with a null draft. `POST` to that endpoint accepts the closed `ProfileDraft` DTO
and returns the same commit-pinned preview contract used by bootstrap. Both are
private-network planning operations; neither writes GitHub nor persists drafts.

The installed adapter supports only `codex-thorough`, `hub-agent-codex` and exact
`gpt-6.1-sol/high`. Name, description, enabled state, permitted context sources
and trusted review-continuation policy can change explicitly. Role, adapter,
model, trigger, authority, validation, image and pipeline policy stay fixed.
Unsupported existing fixed policy and malformed catalogs require review on
GitHub; the editor cannot silently repair or replace them. Image context and
pipeline execution remain disabled; review-continuation execution remains AW-016.

The infrastructure adapter builds the complete profile from canonical templates,
validates it with the canonical AW-003 schema, and merges only selected editable
fields. Other profiles and surrounding comments/instructions remain intact.
Unchanged catalogs retain their bytes; context order is normalized for stable
reviews. Catalog bounds and instruction conflicts still apply. Editing and
canonical bootstrap use distinct digest domains so retries cannot confuse their
publication intent. Identity reconstructs each edited preview from structured
choices and checks the reviewed digest/base inside its authorization boundary.

The `ProfileDraft` transport field shapes are exported from the canonical profile
schema; Identity's contract references this DTO and uses generated transport code.
Run the root contract generators/checks after changes. Unit and API tests cover
explicit disabling/editing, unchanged/idempotent merges, unrelated-policy
preservation, schema dependencies, rejected overrides, safe diagnostics and
fresh-base digests. No additional configuration or Compose dependency is needed.

## Issue assignments and activity (AW-014)

`GET /v1/repositories/{owner}/{repo}/issues/{number}/assignment` projects an
issue, the current canonical profile, and its ten newest exact v1 assignment
comments. It excludes PR sources and bot-authored requests. Open, unlocked issues
with an enabled validated profile can proceed to review. Existing requests keep
Hub focused on their original workflows; making new work after failure requires
an explicit new GitHub request, not automatic replay through Hub.

Every load reads GitHub. Comment history is bounded at 2000 objects; incomplete
history fails closed. Workflow history is bounded at 1000 objects and elects the
earliest matching repository/comment title, workflow path, event and user actor.
Current-attempt job state distinguishes skipped execution from workflow success.
Unavailable evidence remains visible with original request/workflow links.

Linked proposals require matching source/request/profile/user/run provenance,
GitHub Actions bot ownership, same-repository PR head/base, live assignment ref,
commit parent/message/attribution, and the original successful hosted/isolated
jobs. The reader verifies immutable invocation/proposal artifact origin, archive
digests, canonical AW-012 result schema, and patch/summary digests. Storage downloads
use a separate unauthenticated HTTPS client without redirects. It reads files in
memory; no source is executed or applied. The result schema snapshot is generated
from `ops/private-runner/result.schema.v1.json`, with drift checked by the existing
contract exporter.

The publication check must belong to the GitHub Actions App, the exact assignment
and head, with the expected result-derived summary/conclusion. Failed or unavailable
repository validation stays **neutral**, never validated success. A missing check,
expired artifact, moved/deleted branch, altered PR or incomplete evidence needs
GitHub inspection. Displayed context never grants execution/publication authority
or certifies later PR changes. Summary text is fixed recovery guidance or validated
check outcome, never a provider transcript or arbitrary artifact prose.

This GET has a 35-second deadline. There is no runner connection, polling loop,
queue, transcript or durable assignment state. Identity consumes this public
projection without user credentials and owns the subsequent live-authorized
comment write. Controlled HTTP/TLS tests cover source/history boundaries,
canonical ownership, forgery, digest/origin mismatches, neutral validation,
missing evidence and freshness. Use the existing Go/check commands above.
