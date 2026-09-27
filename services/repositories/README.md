# Repositories service

The repositories service owns repository discovery and the user's workspace selection. It reads
repository metadata from GitHub and stores only selected GitHub repository IDs locally. GitHub is
therefore still the source of truth for names, visibility, descriptions, URLs, and access.

The service is layered as follows:

- `internal/domain` contains repository models, ports, and selection use cases.
- `internal/api` maps the versioned HTTP contract to domain operations.
- `internal/infrastructure/github` implements account repository discovery with `go-github`.
- `internal/infrastructure/sqlite` implements the selection store behind the domain port.

Replacing SQLite with PostgreSQL or another database requires a new implementation of
`domain.SelectionStore`; neither the use cases nor HTTP handlers depend on SQL types.

## Local setup

Requirements: Go 1.25 or newer and a GitHub token with read access to the repositories that should
be available for selection.

```sh
cp .env.example .env
set -a
source .env
set +a
go run ./cmd/server
```

Configuration:

| Variable | Default | Purpose |
| --- | --- | --- |
| `GITHUB_TOKEN` | required | Server-only GitHub token identifying the account. |
| `REPOSITORIES_ADDR` | `127.0.0.1:8080` | HTTP listen address. |
| `REPOSITORIES_DB_PATH` | `./data/repositories.db` | SQLite database path. |

The token is never accepted from or returned to the browser. This initial service instance serves
one configured GitHub account; a future login service can supply an account-scoped GitHub client
without changing the repository domain.

The API does not yet authenticate its own callers. Keep it on the loopback or a private application
network and let the SvelteKit server be its only client. The container explicitly listens on all
interfaces so it can be reached on its private container network; do not publish it directly to the
internet.

## Commands

```sh
go test ./...
go vet ./...
gofmt -w .
```

Build and run the container with a persistent data directory:

```sh
docker build -t hub-rearranger-repositories .
docker run --rm -p 127.0.0.1:8080:8080 \
  -e GITHUB_TOKEN \
  -v repositories-data:/app/data \
  hub-rearranger-repositories
```

## Public API

The source-of-truth contract is [`api/openapi.yaml`](api/openapi.yaml).

- `GET /health` — liveness status.
- `GET /v1/repositories` — selected repositories, resolved from GitHub.
- `GET /v1/repositories/available` — repositories associated with the configured account.
- `PUT /v1/repository-selection` — replace the selected repository IDs.
