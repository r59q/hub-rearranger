# Identity service

The identity domain owns Hub's GitHub App user sign-in, encrypted credentials,
server-side sessions, and current repository write authorization. It does not use
`GITHUB_TOKEN`, own repository selections/profiles, or execute agent workloads.
SvelteKit owns the account interface and forwards browser authentication requests
through server routes. GitHub user and refresh tokens never enter public DTOs,
browser storage, cookies, logs, or another service's database.

## Configuration and development

Requirements: Go 1.25. Run from this directory:

```sh
go run ./cmd/server
go test ./...
go vet ./...
```

With no App settings the service starts normally: `/health` is healthy, session
status is `disabled`, and sign-in fails closed. Other application views continue
working. To enable sign-in, configure all four App/vault settings together:

| Setting | Purpose / default |
| --- | --- |
| `AGENTS_API_URL` | Private Agents base URL; `http://127.0.0.1:8082`, `http://agents:8082` in Compose. Used only for bootstrap. |
| `IDENTITY_ADDR` | Listener; `127.0.0.1:8083`. |
| `APP_ORIGIN` | Exact public origin; `http://localhost:3000`. No path/query/userinfo. |
| `IDENTITY_DB_PATH` | Private SQLite database; `data/identity.db`. |
| `GITHUB_APP_ID` | Numeric GitHub App ID; used to match installations. |
| `GITHUB_APP_CLIENT_ID` | App client ID, distinct from its numeric ID. |
| `GITHUB_APP_CLIENT_SECRET` | App OAuth client secret, server-only. |
| `IDENTITY_ENCRYPTION_KEY` | Base64 encoding of 32 random bytes, stable across restarts. |

Generate the vault key locally, store it in the ignored environment/secret store,
and do not paste its value into logs or issue/PR content:

```sh
openssl rand -base64 32
```

The root Compose stack supplies these settings and a private `identity-data`
volume. It does not require an App to run the rest of the application. The image
runs as a non-root user with a writable `/app/data` directory. The API port binds
to loopback for local debugging; keep it private in deployment and serve browser
routes through SvelteKit at the configured public origin.

HTTP is accepted only for a loopback origin during development. Use HTTPS for
other hosts. Configure SvelteKit's `ORIGIN` to exactly the same origin and register
`APP_ORIGIN/auth/callback` as the GitHub App callback URL. Forward the public host
and scheme correctly through the reverse proxy; do not log callback query strings
or cookie/authorization headers. Deployment under a URL path prefix is not
supported by this initial identity contract.

## GitHub App setup and permissions

Register a GitHub App, enable its web authorization flow, and configure the exact
callback URL. User authorization requests no OAuth scopes and uses S256 PKCE.
Enable expiring user access tokens; the adapter also tolerates non-expiring user
tokens, bounded by the Hub session's seven-day lifetime. Install the App only on
repositories whose users need these actions.

Request these repository permissions, with no organization/account permissions:

| Permission | Required operation |
| --- | --- |
| Metadata read (automatic) | Identity/repository role and access checks. |
| Issues write | Assignment comments on issues (AW-014). |
| Contents write | Bootstrap branch/file changes (AW-009). |
| Pull requests write | Bootstrap draft PR creation (AW-009). |
| Workflows write | Bootstrap PRs that add/update workflow files (AW-009). |

Actions write, Administration write, secrets, and merge/release authority are not
required. The existing read-only discovery token stays separate. GitHub App user
tokens act as the user within the intersection of user and installed App access;
installation tokens, personal access tokens, and OAuth App tokens cannot be used
as Hub user identity.

See GitHub's [user authorization guide](https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/generating-a-user-access-token-for-a-github-app)
and [refresh guide](https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/refreshing-user-access-tokens).
Live verification requires a real configured App and its installation; all
repository tests use controlled local HTTP boundaries and synthetic credentials.

## Session and credential lifecycle

Sign-in is an exact-origin POST. A ten-minute, one-use login intent stores the
PKCE verifier and a hash of an independent browser binding. The callback requires
that HttpOnly binding cookie and matching state before exchanging a code. It
verifies the token against this App and retrieves the actual GitHub user; no
claimed login or requester from the browser is trusted. A successful callback
creates a fresh opaque session and invalidates the previous local session.

Cookies contain random 256-bit opaque identifiers only. They are host-only,
HttpOnly, SameSite=Lax, and Path=/; HTTPS uses `__Host-hub_session` and
`__Host-hub_login` with Secure. Loopback HTTP development uses `hub_session` and
`hub_login`. Duplicate cookies fail closed for authenticated operations. Session
responses contain only the verified user, expiry, and a session anti-forgery nonce.
Every POST also requires the exact configured Origin; authenticated POSTs require
the matching nonce. Responses and rendered application pages use `no-store`.

Session lifetime is an absolute seven days. Each authenticated read/authorization
rechecks token validity and user identity. Access tokens are refreshed shortly
before expiry; GitHub's new access/refresh pair replaces the old pair atomically.
A transient provider outage preserves the encrypted record while reporting
unavailable. Expired refresh/session credentials, revoked tokens, or changed user
identity invalidate the local session and require reconnecting. Login intents and
expired sessions are purged during new sign-ins; accessed expired sessions are
removed immediately.

Sign-out checks Origin and the session nonce, deletes local authority first, then
requests GitHub token revocation. If access has expired but its refresh token is
usable, it rotates after local deletion and revokes the current access token.
Failed or ambiguous remote revocation never restores the Hub session; the UI
links to GitHub account settings for manual App-authorization revocation. Automatic
Hub session expiry removes local authority and does not promise remote revocation.

Encrypted records use standard-library AES-256-GCM with a fresh random nonce and
associated data binding each ciphertext to its table/version/hashed identifier.
The SQLite file is owner-only; keys live outside it. Keep the database volume and
key together across restarts. Changing the key makes existing sessions unreadable
and requires sign-in again. If rotating after credential compromise, revoke the
App authorization on GitHub as well; changing a local key alone does not revoke
GitHub credentials. Rotate App client secrets in GitHub and restart this service
with the updated secret.

## Public API and future write consumers

The versioned contract is [`api/openapi.yaml`](api/openapi.yaml):

- `GET/HEAD /health` — liveness, including disabled sign-in.
- `GET /v1/session` — safe status, current identity, and session nonce.
- `POST /v1/sign-in` — GitHub authorization redirect and binding cookie.
- `GET /v1/sign-in/callback` — verified callback, session cookie, fixed account redirect.
- `POST /v1/sign-out` — local removal and remote revocation outcome.
- `POST /v1/repositories/{owner}/{repo}/authorization` — live preflight for
  `assign_comment` or `bootstrap_pull_request`, requiring Origin/session/nonce.

Assignment authorization requires the exact current `maintain` or `admin` role
and Issues write. Bootstrap requires `write`, `maintain`, or `admin` plus Contents,
Pull requests, and Workflows write. Custom role names are conservatively rejected.
Checks read the configured App's current installation permissions, suspension,
and repository selection even for public repositories. Archived/disabled or
renamed repositories fail closed. Pagination and request deadlines are bounded;
missing/denied access and transient errors have different safe recovery responses.

The public preflight result is **not a reusable grant**. AW-009 and future AW-014
mutations must call `domain.Service.WithAuthorization` at the actual write
boundary, executing a typed, repository-bound GitHub operation within the identity
service with the freshly verified user's token. Other domains prepare their own
business payloads and call versioned identity APIs for those atomic user writes;
never export a token or trust an earlier preflight to authorize a later write.
AW-009 adds the bootstrap endpoint below. AW-019 established this authorization
boundary and proved comment attribution using a controlled GitHub HTTP server;
production assignment remains AW-014.

SQLite serializes refresh, sign-out, and authorized operations through a transaction
on a single connection. Run one identity replica with its own volume. This
intentionally favors clear revocation/rotation ordering over high throughput;
a future multi-replica deployment needs an equivalent shared transactional store.
A successful refresh persists even when later repository authorization fails,
because rolling it back would retain a pair GitHub has already invalidated.

`golang.org/x/oauth2` handles the OAuth/PKCE and refresh protocol; `go-github`
handles REST, token checks/revocation, and pagination. SCS was evaluated for general
HTTP session management. This domain instead needs encrypted credential records,
one-use intents, and a transaction spanning refresh and live write authorization,
so its narrow store implements those use cases without a general HTTP session bag.

## Contracts and checks

Go transport types/handlers and server-only frontend types are generated with
the same pinned tools as Agents. From the repository root:

```sh
make generate-identity-contract # after editing OpenAPI
make identity-contract-check   # verifies without editing
make check                     # all components, including identity
```

Tests exercise forgery/replay, session rotation/expiry/revocation, exact roles and
App permissions, safe errors and DTOs, encrypted restart persistence, concurrent
refresh ordering, public API responses, and user-attributed controlled comment
writes. No test requires live GitHub or runner/provider authentication.

## Bootstrap draft PR publication (AW-009)

`POST /v1/repositories/{owner}/{repo}/bootstrap-pull-request` accepts only the
session anti-forgery nonce, reviewed `base_revision` and `digest`. It requires the
exact configured Origin and opaque session cookie. It executes entirely within
`WithAuthorization`, obtains the current canonical file plan from Agents without
forwarding credentials/cookies, and rechecks App-bound user identity, repository
role and Contents/Pull requests/Workflows write access before each GitHub write
and completion. Sessions remain locked across the operation so sign-out/token
refresh cannot race a write. Tokens stay in this service.

Publication creates file/tree objects, one user-attributed commit, a deterministic
`hub-bootstrap/codex-thorough-<user-id>-<base-prefix>-<digest>` branch and an open
draft PR. Only canonical bootstrap paths may change. The publisher independently
verifies the entire proposed tree, including unchanged unrelated files, commit
parent/author/message, actual ref and exact user-owned PR metadata. It cannot
update/delete refs or merge. The PR links to the selected Hub setup and contains
the manual runner/isolation/exact-policy/readiness/enablement checklist.

Retries reconcile actual GitHub refs, commits, trees and PRs, including lost HTTP
responses or a branch whose PR was not created. Concurrent attempts converge on
that same result; no hidden publication state is saved. Changed default branches,
altered branches/PRs, closed PRs and ambiguous results fail closed. HTTP 409
`bootstrap_stale`/`bootstrap_conflict` asks for a fresh review;
`bootstrap_incomplete` asks the user to reload and reconcile. An unreachable GitHub
response is never proof of success. If only an unreachable Git object was created,
a retry may create another unreachable object before publishing the one branch.
The request is bounded to 120 seconds and propagates cancellation; the normal
identity request deadline stays 20 seconds. Agents outages affect bootstrap only.

Controlled API/GitHub tests cover user authority, forgery, malformed plans, stale
bases, unsafe changes, exact tree preservation, phase-specific revocation,
concurrent duplicate attempts, lost responses and partial publication. Run
`go test -race ./...` for the session/publication concurrency checks. The public
Identity contract and the consumed Agents DTOs are generated using the root pinned
tooling; `make check` checks drift. Live use requires the configured real App and
installation described above, and operator runner setup remains manual.

## Profile configuration publication (AW-010)

The existing `bootstrap-pull-request` endpoint optionally accepts `profile_draft`,
a structured authoring DTO referenced from Agents' public OpenAPI contract. It
accepts no browser-authored files or runtime overrides. Missing/null/unknown draft
fields fail before publication. Identity carries the choices to Agents using a
credential-free POST, independently obtains the fresh generated plan, and requires
its exact reviewed base/digest. Agents owns schema and fixed adapter policy;
Identity owns authorization, user attribution and GitHub reconciliation.

Profile edits use the same `WithAuthorization`, per-phase live user/App checks,
full-tree verification, deterministic branch, draft PR and no-ref-update rules
as bootstrap. Their review digests distinguish the authoring intent; the draft
PR links to `/agents/editor` and explains policy changes plus the manual runner
checklist. The publication tests cover edited choices, stale reviews, required
fields, current access, lost responses and duplicate recovery. Regenerate both
Agents and Identity contracts when this shared transport input changes. Local
HTTP tests resolve only the checked-in referenced OpenAPI specification.
