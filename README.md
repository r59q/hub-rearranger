# Hub Rearranger

Hub Rearranger reorganizes GitHub information and workflows into focused views for people and AI
agents. GitHub remains the source of truth; Hub Rearranger stores only application-specific state.

## Components

- [`frontend`](frontend/README.md) — SvelteKit user interface.
- [`services/repositories`](services/repositories/README.md) — repository discovery and selection
  service.
- [`services/issues`](services/issues/README.md) — recently active GitHub issues and relationship
  discovery service.
- [`services/agents`](services/agents/README.md) — repository profile validation and agent convention read API.

The planned GitHub-native agent workflow, its [assignment comment
convention](AGENTIC_WORKFLOWS_ANALYSIS.md#aw-002-assignment-convention-v1), and the
[v1 profile/adapter contract](ops/agent-profiles/README.md) are documented separately.
The repository declares `codex-thorough` in `.github/agent-profiles.yml`;
assignment execution is not installed yet.

## Local development

Docker Compose runs the complete application, including persistent SQLite storage. Copy the root
environment example, provide a fine-grained GitHub token with repository read access, and start the
stack:

```sh
cp .env.example .env
# Edit .env and set GITHUB_TOKEN.
docker compose up --build
```

Open `http://localhost:3000`. The repositories, issues, and agents APIs are also available for local
debugging at `http://localhost:8080`, `http://localhost:8081`, and `http://localhost:8082`; all ports bind to loopback only.
Repository selections persist in the `repositories-data` Docker volume. Use `docker compose down` to stop the application, or
`docker compose down --volumes` to also remove the local selection database.

To run the components directly instead, start the repository service with a GitHub token that can
read the repositories you want to select (Metadata, Contents, and Issues):

```sh
cd services/repositories
GITHUB_TOKEN=github_pat_... go run ./cmd/server
```

Start the issues service in another terminal with the same token:

```sh
cd services/issues
GITHUB_TOKEN=github_pat_... go run ./cmd/server
```

Start the Agents service in another terminal with read-only Contents access
for private repository profiles:

```sh
cd services/agents
GITHUB_TOKEN=github_pat_... go run ./cmd/server
```

Then run the frontend:

```sh
cd frontend
npm ci
npm run dev
```

The frontend defaults to `http://127.0.0.1:8080` for repositories, `http://127.0.0.1:8081` for
issues, and `http://127.0.0.1:8082` for agents. Set the server-only `REPOSITORIES_API_URL`,
`ISSUES_API_URL`, and `AGENTS_API_URL` variables to use other addresses. The Agents adapter is
available for profile reads and future readiness views; existing views do not depend on it.

## Repository-wide validation

Install the combined [profile and runtime development tools](ops/agent-profiles/README.md#offline-validation-and-development)
once before running all checks. The runtime and offline contract tooling live outside Docker Compose.

```sh
make check
```

`make check` also verifies the generated Agents Go/frontend contract code without changing files.
After editing `services/agents/api/openapi.yaml`, run `make generate-agents-contract`.
The GitHub-hosted Application checks workflow runs the same full suite without runner authentication.
