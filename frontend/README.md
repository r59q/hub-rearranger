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
service addresses are needed. GitHub credentials belong to the Go services and must never be
placed in the frontend environment.

The workdesk uses route-backed tabs. `/` is the dashboard for repository selection and recent
activity, while `/issues` is the focused, paginated issues view.

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

Mutations use SvelteKit form actions and `use:enhance` so they retain progressive enhancement and
server-only service access. Shared optimistic-update lifecycle behavior lives in
`src/lib/forms/optimistic-submit.ts`; keep optimistic state local to the affected view and reconcile
it with the authoritative page data returned after the action.
