# Agents service (AW-004 / AW-005 / AW-006)

The Agents service owns repository profiles, their validation, derived readiness,
and the GitHub-native assignment convention. It has no queue, runtime, provider
credential store, transcript store, or durable assignment/profile database.
GitHub remains the source of truth. Runners and their authentication live outside
Hub and Docker Compose. Readiness derives configuration and trusted diagnostic evidence; profile validation alone
never proves that a runner, model, or account is ready.

## Local setup and configuration

Use Go 1.25. Public catalogs can be read without a token; private repositories
require a fine-grained token with read-only Metadata and Contents access. Readiness also requires Actions read access:

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

[`api/openapi.yaml`](api/openapi.yaml), version 1.2.0, uses OpenAPI 3.0.3.
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
