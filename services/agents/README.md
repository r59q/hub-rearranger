# Agents service (AW-004)

The Agents service owns repository profile semantics, derived readiness, and
the GitHub-native assignment convention. It has no queue, runtime, provider
credential store, transcript store, or durable assignment/profile database.
GitHub remains the source of truth. Runners and their authentication live
outside Hub and Docker Compose.

This scaffold exposes liveness and static AW-002/AW-003 convention metadata.
Repository profile reads/validation are AW-005; runtime readiness is AW-006.
The service does not report an empty catalog or a verified runner before those
capabilities exist. Describing an assignment convention does not install its
GitHub workflow, authorize an actor, or write an assignment.

## Local setup and configuration

Use Go 1.25. No GitHub token, provider login, database, or runner is needed:

```sh
go run ./cmd/server
```

`AGENTS_ADDR` defaults to `127.0.0.1:8082`. Set it to `:8082` in a private
container network. Root Docker Compose builds this service and publishes its
port on loopback, with an `AGENTS_PORT` override. Copy `.env.example` if a
different direct-run listen address is needed; environment files are ignored
and never loaded automatically by the service.

The API has no caller authentication at this stage. Its only data is public
protocol metadata. Keep it on loopback or a private application network and
access it through SvelteKit's server adapter. Do not publish it as a public
service. Future repository integrations require deliberate credential and
authorization changes; these scaffold endpoints need neither.

## Layers and integration points

- `cmd/server` composes the application, HTTP timeouts, logging, and signal-based shutdown.
- `internal/api` maps domain results to generated DTOs and returns safe transport errors.
- `internal/domain` owns convention rules, with no HTTP, OpenAPI, or infrastructure dependencies.
- `internal/infrastructure/config` reads process configuration. GitHub adapters will live in infrastructure when introduced.
- `api/openapi.yaml` is the versioned public API contract.
- `frontend/src/lib/server/agents-api.ts` is the typed server-only consumer.

The domain receives request cancellation and creates fresh convention values
for every read. It stores no mutable cross-request policy. Health means process
liveness; it is independent of GitHub, profiles, and external runners.

## Public API

The source of truth is [`api/openapi.yaml`](api/openapi.yaml), version 1.0.0.
It uses OpenAPI 3.0.3 for the pinned generator/validator compatibility path;
the API URL version and catalog schema version are separate contracts.

- `GET /health` returns `{"status":"ok"}`.
- `GET /v1/assignment-convention` describes convention v1, catalog schema v1,
  issue-comment assignment, required current `maintain`/`admin` roles,
  `branch-draft-pr` authority, and a full default-branch ancestor SHA.
- `HEAD` on either endpoint checks availability with no response body.

The returned command template has `{profile_id}` and `{profile_revision}`
placeholders; it is documentation, not an assignment request or runnable shell
command. The profile catalog belongs to the selected repository and will be
read by AW-005, never inferred from this service's local checkout.

Unsupported methods return `405` with `Allow: GET, HEAD`. Unrecognized paths
return `404`. GET error responses use the contract's `Error` schema with a safe
`code` and `message`; internal failures return `500`. Exceptions and arbitrary
request data are excluded from responses and logs. The frontend distinguishes
invalid responses from service unavailability using safe adapter errors.

## Development, contracts, and tests

Use the repository-wide formatter/linter versions and commands. From this
component:

```sh
gofmt -w cmd internal
go vet ./...
go test ./...
```

From the repository root, after the documented frontend and Python setup:

```sh
make generate-agents-contract
make agents-contract-check
make check
```

The pinned `oapi-codegen` generates Go DTOs and standard-library HTTP routes;
`openapi-typescript` generates frontend types and enum values, consumed through
a focused `openapi-fetch` adapter. Domain types are mapped at the API boundary.
Generated transport code is required checked-in contract code, not a domain
model; regenerate it rather than editing it. Generator options and versions
are pinned in `api/codegen.yaml`, the root Makefile, and the frontend lockfile.

Contract checks regenerate into a temporary directory and compare without
editing the working tree. API tests validate actual HTTP requests/responses
against OpenAPI with `kin-openapi`, including safe failure responses and
method rejection. Domain tests cover assignment constraints, cancellation,
and mutation isolation. Frontend tests cover the server-only address,
successful reads, malformed policies, and unavailable/error responses.
No test needs GitHub, OpenAI, authentication, or a self-hosted runner.

Generated API types do not perform runtime validation: the frontend additionally
checks the response shape and supported policy values before returning data.
Expected enum values come from the generated contract. Profile YAML parsing and
validation continue to use the separate AW-003 schema; this scaffold does not
reimplement that schema or install a Python runtime in the Go service.
