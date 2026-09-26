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

The repository workspace is rendered with data from the Go repositories service. It uses the
server-only `REPOSITORIES_API_URL` setting, which defaults to `http://127.0.0.1:8080`. Copy
`.env.example` to `.env` when a different service address is needed. GitHub credentials belong to
the Go service and must never be placed in the frontend environment.

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
service. Route load functions and actions use this adapter on the server; browser components
receive application-level repository data and never call the Go service directly.
