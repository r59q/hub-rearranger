# Hub Rearranger

Hub Rearranger reorganizes GitHub information and workflows into focused views for people and AI
agents. GitHub remains the source of truth; Hub Rearranger stores only application-specific state.

## Components

- [`frontend`](frontend/README.md) — SvelteKit user interface.
- [`services/repositories`](services/repositories/README.md) — repository discovery and selection
  service.
- [`services/issues`](services/issues/README.md) — recently active GitHub issues and relationship
  discovery service.
- [`services/agents`](services/agents/README.md) — repository profiles, readiness diagnostics, and agent convention read API.
- [`services/identity`](services/identity/README.md) — GitHub App user sign-in, encrypted sessions, and repository write authorization.

The planned GitHub-native agent workflow, its [assignment comment
convention](AGENTIC_WORKFLOWS_ANALYSIS.md#aw-002-assignment-convention-v1), and the
[v1 profile/adapter contract](ops/agent-profiles/README.md) are documented separately.
The repository declares `codex-thorough` in `.github/agent-profiles.yml`.
The [trusted assignment intake](ops/agent-intake/README.md) is implemented;
dedicated patch execution and publication remain disabled until AW-012/AW-013.

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
`docker compose down --volumes` to also remove the local selection database and identity session store.

To run the components directly instead, start the repository service with a GitHub token that can
read the repositories you want to select (Metadata, Contents, Issues, and Actions):

```sh
cd services/repositories
GITHUB_TOKEN=github_pat_... go run ./cmd/server
```

Start the issues service in another terminal with the same token:

```sh
cd services/issues
GITHUB_TOKEN=github_pat_... go run ./cmd/server
```

Start the Agents service in another terminal with read-only Metadata, Contents, and Actions access
for repository profiles and readiness diagnostics:

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
`ISSUES_API_URL`, and `AGENTS_API_URL` variables to use other addresses. Open the Agents tab or a
repository card's Agent profiles link to review profile policy, configuration diagnostics, and
runtime readiness. GitHub source links and refresh controls keep the view tied to repository
state. An Agents outage leaves the dashboard and issues view available.

The header's GitHub account link opens `/account`. User sign-in is optional until you configure
a GitHub App using the [identity setup guide](services/identity/README.md#configuration-and-development).
The identity API defaults to `http://127.0.0.1:8083`; SvelteKit uses server-only `IDENTITY_API_URL`.
Compose includes the identity service and its encrypted `identity-data` volume. App credentials
and the vault key are separate from `GITHUB_TOKEN`; browsing works while sign-in is unconfigured.
Live sign-in needs an App callback matching `APP_ORIGIN/auth/callback` and an installation with
the action-specific permissions documented in the guide.

## Repository-wide validation

Install the combined [profile and runtime development tools](ops/agent-profiles/README.md#offline-validation-and-development)
once before running all checks. The runtime and offline contract tooling live outside Docker Compose.

```sh
make check
```

`make check` also verifies the generated Agents Go/frontend contract code without changing files.
After editing `services/agents/api/openapi.yaml`, run `make generate-agents-contract`.
For `services/identity/api/openapi.yaml`, use `make generate-identity-contract`; its non-mutating
contract check is also included in `make check`.
The GitHub-hosted Application checks workflow runs the same full suite without runner authentication.
`make intake-check` verifies the assignment intake and controlled GitHub boundaries
without contacting GitHub or starting a credentialed runner.
