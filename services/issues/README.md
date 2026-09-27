# Issues service

The issues service owns the read-only GitHub issues domain for Hub Rearranger. It lists open issues
for repository identities supplied by the frontend, orders them by most recent GitHub activity, and
enriches the visible page with GitHub sub-issues and timeline cross-references. It stores no issue
data; GitHub remains the source of truth.

The service is intentionally independent from the repositories service. The SvelteKit server reads
the current selection from repositories and passes only `owner/name` identities to this public API.

## Local setup

Requirements: Go 1.25 and a fine-grained GitHub token with read-only Metadata and Issues access for
the repositories you want to display.

```sh
cp .env.example .env
set -a
source .env
set +a
go run ./cmd/server
```

The service listens on `127.0.0.1:8081` by default. Set `ISSUES_ADDR` to override the address.
Credentials are read from `GITHUB_TOKEN`; never expose that value to the frontend.

The API does not authenticate its own callers. Keep it on loopback or a private application
network and use the SvelteKit server as its client; do not publish the service directly.

## Commands

```sh
gofmt -w .
go vet ./...
go test ./...
```

## Public API

The versioned contract is [`api/openapi.yaml`](api/openapi.yaml).

- `GET /health` — liveness response.
- `GET /v1/issues?repository=owner/name&page=1&per_page=6` — open issues ordered by recent
  activity. Repeat `repository` for each selected repository.

The response reports repositories whose issues could not be read and relationship details that
could not be loaded. If every selected repository fails, the endpoint returns a user-safe `502`.

## Integration points

- GitHub REST API via `go-github` for issue lists, sub-issues, and timeline cross-references.
- The frontend server adapter at `frontend/src/lib/server/issues-api.ts`.
