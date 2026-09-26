# Repository guidance

## Project state

Hub Rearranger is a frontend wrapper for GitHub. It reorganizes GitHub
information and workflows to make them easier for AI agents and their human
operators to use.

The product should reduce the cognitive and interaction overhead of common
GitHub tasks while preserving the underlying GitHub source of truth. Establish
the project structure, tooling, and conventions deliberately as features are
introduced. Update this file when those decisions become stable enough to guide
future work.

The backend is written in Go and follows a microservices architecture. The
frontend is a SvelteKit application. Keep their responsibilities distinct: Go
services own GitHub authentication, authorization, integration, and
domain-specific server-side business logic; SvelteKit owns the accessible,
responsive user interface and its view-specific state.

Go services live under `services/<domain>` as independent modules. Use
`cmd/server` for composition, `internal/api`, `internal/domain`, and
`internal/infrastructure` for the service layers, and `api/openapi.yaml` for the
versioned public contract. Frontend calls to those services belong in typed
`src/lib/server` adapters so service addresses and credentials stay server-side.

## Product principles

- Build focused views around agent workflows: understanding repository state,
  selecting work, making changes, reviewing results, and handing work off.
- Show the context needed to make a decision together, rather than scattering
  it across GitHub pages and tabs.
- Make status, ownership, required next actions, and links to source GitHub
  objects easy to identify.
- Keep the interface simple and predictable. Prefer progressive disclosure over
  dense pages or duplicate controls.
- Preserve a clear path to the original GitHub object for users who need its
  full capabilities.

## GitHub integration and data

- Treat GitHub as the source of truth. Do not maintain conflicting local state
  for issues, pull requests, repositories, or workflow runs.
- Use the established `go-github` client library for GitHub REST API access.
  Extend it through its supported mechanisms before writing custom GitHub HTTP
  clients, request signing, pagination, or response decoding.
- Request the smallest practical GitHub permission scope and explain any action
  that writes to GitHub before it is performed.
- Never expose access tokens, OAuth credentials, webhook secrets, or private
  repository data in client bundles, logs, test fixtures, or version control.
- Design read operations to tolerate missing permissions, deleted resources,
  rate limits, and API errors with useful recovery guidance.
- Make mutating GitHub actions deliberate, clearly named, and easy to review.

## Working practices

- Inspect the repository before making changes and preserve unrelated work.
- Keep each change focused on the requested outcome; avoid speculative features
  and broad refactors.
- Keep source files small, neatly decomposed, and easy to read. Split files into
  focused modules or components when they become large or take on multiple
  responsibilities. Refactoring oversized code encountered while working is
  allowed and preferred even when that code is not directly related to the
  requested change, provided behavior is preserved and unrelated user work is
  not overwritten.
- Maintain a Docker Compose file at the repository root that contains everything
  needed to run the complete application locally for testing and debugging.
  Update the Compose configuration whenever services, dependencies,
  configuration, ports, volumes, or local-development requirements change.
- Prefer clear, conventional names and small, composable modules.
- Keep GitHub API access behind a narrow, typed integration layer. UI components
  should consume application-level data rather than raw API responses.
- Make asynchronous states explicit: loading, empty, error, stale, and
  success states should each be understandable without developer tools.
- Prefer optimistic UI responses for predictable, reversible user actions so
  successful interactions feel immediate. Clearly represent pending state,
  reconcile it with the authoritative response, and roll it back with useful
  error feedback when the operation fails. Avoid optimistic updates for
  destructive or high-risk actions where showing unconfirmed success could
  mislead the user.
- Build accessible keyboard-friendly interactions and use semantic HTML before
  adding custom controls.
- Document non-obvious design decisions near the code or in project
  documentation.
- Do not add generated files, credentials, local environment files, or build
  output to version control.

## Architecture and service boundaries

- Use domain-driven design to define service boundaries around cohesive product
  domains, such as `issues`, `pull-requests`, and `repositories`. Add other
  services only when they own a distinct domain and lifecycle.
- Each service owns its domain logic, persistence, and integration details. Do
  not let another service read or write its internal data store directly.
- Prefer domain events and versioned public APIs for cross-service workflows.
  Avoid synchronous call chains when an asynchronous handoff preserves the
  required user experience.
- Do not share domain models, database schemas, or business-logic packages
  across services. Shared libraries should be small and limited to stable
  cross-cutting concerns such as observability or transport utilities.
- Keep service deployment, configuration, and failure behavior independent so a
  service can evolve or be unavailable without taking unrelated domains down.

## Layered architecture

- Organize every service into clear API, domain, and infrastructure layers.
  Dependencies must point inward: API and infrastructure layers may depend on
  the domain; domain code must not depend on HTTP, databases, GitHub clients,
  messaging systems, or framework types.
- The API layer owns transport concerns: OpenAPI-derived handlers, request
  parsing, authentication context, input validation, response mapping, and
  status codes. Keep it thin and free of domain decisions.
- The domain layer owns use cases, business rules, domain models, and interfaces
  for capabilities it needs. It must remain independently testable with simple
  fakes or mocks.
- The infrastructure layer implements domain interfaces and owns external
  details such as `go-github`, databases, event brokers, caches, and telemetry.
  Do not allow infrastructure types to leak into API or domain contracts.
- Apply the same separation to the SvelteKit frontend: routes and components
  own UI concerns; frontend application logic owns view use cases and state;
  API clients, storage, and browser integrations remain in infrastructure
  adapters.

## Dependencies and implementation choices

- Prefer mature, well-maintained libraries, frameworks, and tools over custom
  implementations of common infrastructure or application concerns.
- Evaluate an existing project dependency or established ecosystem package
  before creating a new abstraction, utility, protocol client, parser,
  serializer, authentication flow, queue, or UI primitive.
- Keep third-party dependencies behind focused adapters where they touch domain
  logic, so the domain remains testable and a dependency can be replaced
  without spreading its types across the service.
- Choose dependencies that are actively maintained, compatible with the
  project's license, and appropriate for the security and operational needs of
  the service. Pin and update them through the project's normal dependency
  workflow.

## Component documentation and ignore rules

- Every service, the SvelteKit frontend, and each shared library must contain
  its own `README.md`. Keep it current with the component's purpose, ownership
  boundary, local setup, configuration, development commands, test commands,
  and its public API or integration points.
- Every service, the SvelteKit frontend, and each shared library must contain a
  scoped `.gitignore` for its language- and tool-specific generated files,
  build output, local environment files, and test artifacts. Do not use it to
  hide source files or required generated contract code.
- Keep component documentation specific to that component. Put cross-cutting
  architecture, repository-wide tooling, and onboarding material in the root
  documentation instead of duplicating it across component READMEs.

## Code quality standards

- Establish the formatter and linter configuration at the repository level
  before adding multiple services. Apply the same versions, rules, and scripts
  to every Go service; do not let individual services define their own style.
- Go services must use `gofmt` as the baseline formatter and run an
  opinionated, repository-standard static analysis or lint command. Format and
  lint checks must run in local development and continuous integration.
- The SvelteKit frontend must use checked-in, opinionated formatting and linting
  tools. Apply them consistently to Svelte, TypeScript, JavaScript, CSS, and
  configuration files within their supported scope.
- Keep formatter and linter configuration, ignore rules, and package scripts in
  version control. Avoid editor-only conventions that cannot be reproduced in
  continuous integration.
- Treat formatting and lint failures as failures to fix, rather than routinely
  bypassing or suppressing them. Any exception needs a narrow, documented
  reason.

## API contracts

- Define every public service API in an OpenAPI specification. Treat the
  specification as the source of truth for routes, request and response
  schemas, authentication requirements, validation, and error responses.
- Version OpenAPI contracts and review contract changes as carefully as code.
  Preserve backwards compatibility unless a coordinated breaking release is
  explicitly intended.
- Generate or validate server handlers and frontend clients from the OpenAPI
  specifications where practical; do not hand-maintain divergent API types.
- Prefer generating DTOs from the OpenAPI schemas for service handlers and
  frontend clients. Map generated transport DTOs to and from domain models at
  the API boundary; do not use DTOs as domain entities or persist them as
  internal models.
- Give each service a clear external API. Do not expose internal implementation
  endpoints as cross-service dependencies.

## Testing

- Add unit tests for every feature when doing so exercises meaningful behavior
  without creating brittle or difficult-to-maintain test code. Focus on domain
  rules, use cases, transformations, and error handling.
- Use the Arrange–Act–Assert structure in tests: set up only the necessary
  state, perform one behavior, then assert its observable result.
- Add integration tests for each integration and public API behavior. Verify
  that endpoints honor their OpenAPI contract, including authentication,
  validation, success responses, error responses, and relevant side effects.
- Test infrastructure adapters against realistic integration boundaries, such
  as a controlled HTTP server for GitHub API behavior or an isolated database,
  rather than asserting implementation details or mocking the system under
  test.
- Keep tests deterministic and independent. Do not require live GitHub access,
  shared external environments, or test ordering unless an explicitly managed
  integration environment makes that necessary.

## Go backend

- Follow standard Go formatting and package conventions. Run `gofmt` on changed
  Go files and the repository-standard lint checks before handoff.
- Keep HTTP handlers thin. Put workflow and GitHub-facing logic in focused
  services so it can be tested without HTTP transport details.
- Pass `context.Context` through request-bound operations and honor
  cancellation and deadlines in GitHub and other network calls.
- Return structured, user-safe errors. Log enough server-side context for
  diagnosis without logging credentials or private repository contents.
- Implement and validate the service's versioned OpenAPI contract. Make
  compatibility changes intentionally and update affected consumers in the same
  change.

## SvelteKit frontend

- Prefer Svelte components, stores, and SvelteKit load/actions according to
  their intended responsibilities; do not add client-side state libraries
  without a clear need.
- Keep GitHub credentials and privileged operations on the server side. Never
  place secrets in `PUBLIC_` environment variables or browser code.
- Use SvelteKit's server routes or the Go service APIs deliberately. Avoid
  duplicating authorization, validation, or GitHub integration in the browser.
- Type data crossing route, component, and API boundaries. Keep display-specific
  transformations close to the components that use them.
- Support the user's system color-scheme preference. The frontend must provide
  a dark theme when `prefers-color-scheme: dark` is active, while preserving
  accessible contrast and clearly visible interaction states in both themes.
- Treat mobile users as a target demographic. Design and test frontend views
  for small screens, touch input, readable content, and responsive layouts as
  first-class use cases rather than desktop-only adaptations.
- Test user-visible workflows and component behavior with the project's chosen
  test tools once they are established.
- Run the repository-standard frontend formatter and linter before handoff.

## Validation

- Add or update maintainable unit and integration tests with behavior changes.
- Run the relevant formatter, linter, type checker, and test suite before
  handing off a change.
- If validation cannot run, state what was not run and why.

## Git hygiene

- Do not discard, overwrite, or reformat unrelated user changes.
- Keep commits small and coherent when asked to commit.
- Use descriptive commit messages written in the imperative mood.
