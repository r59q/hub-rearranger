# Hub Rearranger frontend

The SvelteKit frontend provides the accessible, responsive workspace for Hub Rearranger. It owns
presentation and view-specific state; GitHub authentication, authorization, and domain logic belong
to backend services.

## Design system

The frontend uses [Pico CSS](https://picocss.com/) for lightweight, semantic component styling.
The Jade theme is loaded globally from the installed npm package, with only small view-specific
styles added in Svelte components. Pico follows the system `prefers-color-scheme` setting, so light
and dark themes do not require client-side JavaScript.

## Local development

Requirements: Node.js 24 and npm.

```sh
npm ci
npm run dev
```

The development server runs on `http://localhost:5173` by default.

The workspace is rendered with data from the Go repositories and issues services. It uses the
server-only `REPOSITORIES_API_URL` and `ISSUES_API_URL` settings, which default to
`http://127.0.0.1:8080` and `http://127.0.0.1:8081`. Copy `.env.example` to `.env` when different
service addresses are needed. The server-only `AGENTS_API_URL` defaults to
`http://127.0.0.1:8082` and supplies the Agents view.
GitHub credentials belong to the Go services and must never be
placed in the frontend environment.

The workdesk uses route-backed tabs. `/` is the dashboard for repository selection and recent
activity, `/issues` is the focused, paginated issues view, and `/agents` shows repository
agent profiles and readiness. Repository cards link directly to their profiles; the Agents
repository selector accepts only selected repositories (`?repository=owner/repo`).

## Agent profiles and readiness

The read-only Agents view brings role, authority, model policy, runner requirements, configuration
diagnostics, and runtime evidence together. Native disclosures reveal execution/review policy and
safe diagnostic details. Catalog, commit, workflow, and diagnostic-attempt links open GitHub.
Missing configuration and pending, verified, or failed runtime verification have distinct labels
and next actions. Repository-service errors, empty selections, missing/invalid catalogs, and
readiness-only failures have separate recovery guidance; a readiness failure keeps profile policy
visible without claiming runtime verification.

`src/lib/server/agents/workspace.ts` maps validated API responses to presentation types in
`src/lib/agents`. Catalog and readiness reads run concurrently and must match repository,
branch, revision, profile set, runner, and requested model policy before showing runtime evidence.
A changed repository snapshot requires a refresh. After five minutes, expired 24-hour evidence,
or a failed refresh, the view suppresses current verification labels. This presentation freshness
rule does not replace the Go service's authoritative evidence validation.

Initial server rendering includes the complete view and works without JavaScript. Client
navigation streams the workspace result with an explicit loading state; refreshing rereads the
services and marks previous results as awaiting refresh. GET forms remain functional without
JavaScript. All service reads stay server-side and responses use `Cache-Control: no-store`.

Unit tests cover snapshot matching, partial failures, freshness, route selection, safe rendering,
and disabled profiles. For browser verification, run the frontend against controlled service
fixtures and check repository selection, refresh/loading, error recovery, keyboard disclosures,
system light/dark themes, narrow touch layouts (including 320px), and JavaScript-disabled forms.

## GitHub account connection

The header's account link opens `/account`, with explicit signed-out, authenticated,
reconnect-required, unconfigured, and unavailable states. Account loading alone calls the
identity service; other views remain independent of its availability. Native POST forms support
sign-in and sign-out without JavaScript and show pending state when JavaScript is available.
The view follows system dark mode and supports keyboard disclosures and narrow touch layouts.

`src/lib/server/identity-api.ts` uses the generated identity contract and maps only safe user,
expiry, and anti-forgery fields to the UI. `IDENTITY_API_URL` defaults to `http://127.0.0.1:8083`.
GitHub App credentials and encrypted token storage belong to the Go identity service; the
frontend environment contains no GitHub user/refresh token or App client secret.

Server routes at `/auth/start`, `/auth/callback`, and `/auth/sign-out` forward only identity
cookies to the private service and preserve its opaque HttpOnly cookies. OAuth redirects are
handled without server-side redirect following; only GitHub's authorization URL and the fixed
account destination are accepted. POSTs require the browser's same Origin; the Go service also
checks Origin and the session nonce. Provider/service prose is never relayed to the UI.
`hooks.server.ts` prevents caching rendered account data, and auth redirects use no-referrer.

Configure the Go service and exact public callback using its [setup guide](../services/identity/README.md).
SvelteKit's `ORIGIN` must match the identity service's `APP_ORIGIN`. The initial identity contract
uses the root URL paths, so deployment under a path prefix is unsupported. After identity API
changes, run `make generate-identity-contract` at the repository root.
Both contract generators run `scripts/format-contract.mjs` before Prettier to
preserve the repository's empty line between type definitions. Contract checks
use the same formatting step; do not edit generated contracts directly.

Tests cover safe response mapping, private cookie forwarding, untrusted redirects, forged or
ambiguous callbacks, sign-out recovery, and expired-session cookie removal. Browser verification
uses synthetic OAuth/REST responses with the real identity service; no live credentials are needed.

## Commands

```sh
npm run check  # Svelte and TypeScript checks
npm run lint   # Prettier and ESLint checks
npm test       # Unit tests
npm run build  # Production build
```

## Container deployment

Build and run the production image from this directory:

```sh
docker build -t hub-rearranger-frontend .
docker run --rm -p 3000:3000 hub-rearranger-frontend
```

The server listens on port `3000` and binds to all interfaces. Place Nginx in front of it as the
public TLS endpoint; proxy application requests to `http://127.0.0.1:3000` (or the container's
network address). The unauthenticated health endpoint is `GET /health` and returns
`{"status":"ok"}`.

For deployments at a public origin, set the SvelteKit `ORIGIN` environment variable to that origin,
for example `https://hub.example.com`. Do not put credentials in frontend environment variables.

## Public integration points

- `GET /health` — liveness endpoint for the container platform or reverse proxy.
- Port `3000` — HTTP server intended to be reached through Nginx.

## Backend integration

`src/lib/server/repositories-api.ts` is the typed infrastructure adapter for the repositories
service. `src/lib/server/issues-api.ts` provides the equivalent issues boundary. Route load
functions and actions use these adapters on the server; browser components receive
application-level data and never call the Go services directly.

`src/lib/server/agents-api.ts` reads the Agents assignment convention and repository profile catalogs, using `openapi-fetch`
and generated OpenAPI types/enum values in `agents-contract.gen.ts`. It validates successful
response shapes and exposes safe service/transport errors. `getRepositoryProfiles(owner, repo)`
returns the catalog state, commit revision, complete typed profile policy, and diagnostics.
Profile semantics are validated by pinned Ajv against the generated copy of the canonical
Draft 2020-12 schema; catalog validation does not establish runtime readiness. Create a client with `agentsClient(fetch)`
inside server load functions. No service address or credentials
belong in browser code. From the repository root, run `make generate-agents-contract` after
editing the Agents OpenAPI specification; `make agents-contract-check` verifies generated code
without modifying it.

Mutations use SvelteKit form actions and `use:enhance` so they retain progressive enhancement and
server-only service access. Shared optimistic-update lifecycle behavior lives in
`src/lib/forms/optimistic-submit.ts`; keep optimistic state local to the affected view and reconcile
it with the authoritative page data returned after the action.

`getRepositoryReadiness(owner, repo)` returns the separate AW-006 readiness
projection, with missing configuration, pending/failed verification, and verified
runtime states, expected runner labels, and next actions. `agents-readiness-response.ts`
checks generated response types and the canonical evidence schema (Ajv with pinned `ajv-formats`), including
state/evidence consistency and safe fixed reason codes. Pending/missing states
carry no untrusted evidence. Reads allow 25 seconds for the service's 20-second
GitHub deadline and give Actions-read permission recovery guidance. This adapter
supplies the repository view through the server-side workspace mapper; it introduces no browser credentials.

## Bootstrap review (AW-009)

From a selected repository in `/agents`, open **Review bootstrap setup**. The
`/agents/bootstrap?repository=owner/name` page obtains a fresh canonical diff and
checks the user's Identity connection on this opt-in page. Other browsing loads
do not depend on Identity. The page explains Git object, commit, dedicated branch
and draft PR writes and manual runner setup before requiring explicit consent.
It renders escaped source in keyboard-scrollable disclosures, supports light/dark
mobile layouts and native no-JavaScript forms, and disables submission while an
enhanced request is pending. Configuration conflicts, unchanged files, unavailable
services, reconnect requirements, stale reviews and verified PR success are explicit.

The bounded, closed same-origin action forwards only the session nonce, base and
review digest through the generated server-only Identity client. Service adapters disable implicit SvelteKit credential forwarding; only Identity
receives the explicitly selected authentication cookies. Neither browser
files nor a discovery token can become a GitHub write payload. Identity obtains
and reauthorizes the plan itself. On success the saved result is GitHub's draft PR;
Hub stores no setup draft. Retrying reconciles that existing result. Tests cover
review mapping/hashes, authorization/recovery, forged forms, native rendering and
safe result links. `npm run check`, `npm run lint`, `npm test` and `npm run build`
validate this component.
