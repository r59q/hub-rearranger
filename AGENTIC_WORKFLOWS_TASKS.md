# Agentic Workflows Task List

> Implementation backlog for the first GitHub-native agent workflow. The
> initial target is `codex-thorough` running through an authenticated,
> subscription-backed Codex CLI on an approved GitHub Actions self-hosted
> runner. Hub Rearranger remains optional throughout.

## Milestone

**GitHub-native `codex-thorough` on an approved runner**

Definition of done: a trusted user can assign one repository issue from
either Hub or GitHub; GitHub Actions runs Codex on the approved runner, produces
a dedicated branch and draft PR, and processes one follow-up PR review comment.
The same workflow remains operable after Hub is removed.

## Coordination rules

- This file is the source of truth for task status, ordering, dependencies, and
  acceptance criteria. Mark a task complete here only after its acceptance
  criteria are met; add a short completion note when useful.
- Task IDs remain stable. Phases group related work; dependencies determine
  which tasks can be picked, including dependencies on later-numbered tasks.
- Give every implementation task one domain owner and one active
  implementation agent. Review-only agents may work in parallel.
- Each implementation change references one task ID in its handoff, commit, or
  pull request where applicable, and avoids unrelated refactoring.
- Before parallel implementation begins, merge the profile schema, assignment
  convention, and runner security contract. Those are shared interfaces.
- Record stable decisions in `AGENTIC_WORKFLOWS_ANALYSIS.md`; do not store them
  only in agent chat.
- No task may place credentials, persisted Codex authentication, or private
  source content in repository files, issue bodies, test fixtures, or logs.
- When manually asked to pick the next task, an agent reads this file and
  `AGENTIC_WORKFLOWS_ANALYSIS.md`, selects the first unchecked task with
  satisfied dependencies, and reports any external or human decision that
  prevents completion.

## Phase 0 — Prove the runtime assumptions

- [x] **AW-001 — Self-hosted runner and subscription-authentication spike**

  **Domain:** external runtime / GitHub Actions  
  **Depends on:** none

  Set up a self-hosted GitHub Actions runner outside the Hub
  Docker Compose stack. Install Codex and establish the selected
  subscription-backed authentication path on that runner. Run a harmless,
  trusted-only diagnostic job against an approved repository.

  **Acceptance criteria:**

  - The runner is restricted to an approved repository or runner group.
  - A GitHub Actions job can confirm runner availability, Codex installation,
    and authentication readiness without printing credentials.
  - The job runs only for current repository maintainers or admins, including
    on a public repository; forked and pull-request contributions cannot run it.
  - The documented setup, rotation, revocation, and failure-recovery procedure
    is reviewed.

  **Completed (2026-09-28):** The approved public repository
  `r59q/hub-rearranger` ran the
  [diagnostic](https://github.com/r59q/hub-rearranger/actions/runs/36456987109)
  at commit `e7711ffcff53d6ab88e0592c60d2291cf23b9e82`. Its GitHub-hosted
  authorization job verified the original/rerun actor's current maintainer role,
  then the repository-scoped `addons` runner passed the pinned probe, Codex
  0.158.0, cached ChatGPT login, and fixed live subscription request. The
  [offline checks](https://github.com/r59q/hub-rearranger/actions/runs/36456987068)
  also passed. The [operating guide](ops/private-runner/README.md) was reviewed
  against this run for setup, rotation, revocation, and failure recovery.
  The operator explicitly relaxed the dedicated host/account and private
  repository requirements for this spike. The shared `r59q` service account
  remains a temporary exception; authorization, default-branch, and no-fork
  guards are required for any later credentialed workflow.

- [x] **AW-002 — Lock the GitHub-native assignment convention**

  **Domain:** agents / GitHub workflow convention  
  **Depends on:** AW-001

  Choose the initial trigger surface for a profile assignment: GitHub App
  action, label, or profile-named issue comment. Define the GitHub-visible
  request/provenance format and the PR provenance format.

  **Acceptance criteria:**

  - The convention identifies source issue/PR, profile ID, profile revision,
    requester, and authority without containing a credential.
  - A person can create and understand the request from GitHub without Hub.
  - The convention identifies the profile owning a resulting PR.
  - The decision is recorded in the analysis document and issue.

  **Completed (2026-09-29):** Chose a new issue comment with an explicit
  profile ID, full default-branch profile commit SHA, and `branch-draft-pr`
  authority. GitHub supplies source and requester identity. Defined the
  acceptance receipt, versioned draft-PR provenance, and live authorization
  checks in the [analysis](AGENTIC_WORKFLOWS_ANALYSIS.md#aw-002-assignment-convention-v1)
  and [issue #3](https://github.com/r59q/hub-rearranger/issues/3).

- [x] **AW-003 — Define the minimal profile schema and adapter contract**

  **Domain:** agents  
  **Depends on:** AW-002

  Define the first version of `.github/agent-profiles.yml` and the contract
  between a profile and a GitHub Actions runtime adapter.

  **Acceptance criteria:**

  - `codex-thorough` can express adapter, model/reasoning policy, triggers,
    context, authority, validation, and continuation policy.
  - The schema excludes credentials and arbitrary unsandboxed command fields.
  - A profile revision can be pinned in an assignment.
  - Invalid profiles have deterministic, user-safe validation errors.

  **Completed (2026-09-30):** Added the repository-local `codex-thorough`
  [catalog](.github/agent-profiles.yml), closed v1
  [JSON Schema](ops/agent-profiles/schema.v1.json), and
  [adapter contract/offline validator](ops/agent-profiles/README.md). Defined
  revision pinning, current-policy revocation, named validation checks,
  patch/write boundaries, and safe deterministic errors. All 23 profile tests
  and the full `make check` passed; checks run without runner authentication.
  Execution workflows, dedicated-runner setup, and verification of the requested
  `gpt-6.1-sol`/`high` policy remain later work.

## Phase 1 — Add the Agents domain read path

- [x] **AW-004 — Scaffold the Agents service and public contract**

  **Domain:** agents  
  **Depends on:** AW-003

  Add `services/agents` with its README, scoped ignore rules, layered Go
  structure, OpenAPI contract, Docker Compose entry, and frontend server-side
  adapter.

  **Acceptance criteria:**

  - The service owns only profile/readiness/assignment-convention concerns.
  - It does not implement a queue, runtime, provider credential store, or
    agent transcript store.
  - Repository-wide checks and Compose continue to run.

  **Completed (2026-09-30):** Added the layered
  [Agents service](services/agents/README.md), versioned OpenAPI contract,
  generated Go handlers and frontend types, and typed server-only frontend
  adapter. The service exposes health and the v1 assignment convention without
  credentials, persistence, or an execution runtime. Added contract-drift checks,
  repository-wide CI, and Compose wiring. Full `make check` passed; all four
  services built and became healthy in an isolated Compose smoke test, and
  stopping Agents left the other services healthy. Profile reads and readiness
  remain AW-005 and AW-006.

- [x] **AW-005 — Read and validate repository-local profiles**

  **Domain:** agents  
  **Depends on:** AW-004

  Read `.github/agent-profiles.yml` from the selected repository and expose
  parsed profiles, profile revisions, and validation results.

  **Acceptance criteria:**

  - Missing, malformed, and unsupported profile files produce actionable
    states rather than generic failures.
  - Tests cover valid `codex-thorough`, invalid schema, and missing-file cases.
  - No profile data is persisted as a competing Hub source of truth.

  **Completed (2026-09-30):** Added fresh default-branch profile reads through
  `go-github`, commit-pinned catalog/profile revisions, full canonical v1 schema
  validation, and safe missing/invalid/unsupported states. The versioned read API
  and typed server-only frontend adapter expose complete validated profiles and
  actionable diagnostics without profile persistence. Contract export/drift
  checks keep Go/frontend schemas and generated transport shapes aligned with
  AW-003. Updated token/Compose wiring and component documentation. Full
  `make check` passed; Agents/frontend images built, and an isolated Agents
  container became healthy. Readiness and the profile UI remain AW-006/AW-007.

- [x] **AW-006 — Derive readiness diagnostics**

  **Domain:** agents  
  **Depends on:** AW-005, AW-001

  Derive “configuration present” from repository files and GitHub workflow
  metadata. Read the no-write GitHub diagnostic result as “runtime verified.”
  Define the profile-specific evidence contract and update the no-write
  diagnostic producer to publish it as a GitHub artifact or check.

  **Acceptance criteria:**

  - The UI/API clearly distinguishes missing configuration, pending runtime
    verification, verified runtime, and failed verification.
  - The diagnosis names missing files, expected runner label, and next action.
  - Authentication is never exposed; only safe readiness status is shown.
  - Versioned, machine-readable evidence identifies the repository, profile ID
    and revision, expected runner label, requested/effective model and reasoning
    policy, CLI version, workflow run/attempt, verification time, and safe outcome.
    The reader verifies its origin against the trusted GitHub workflow/job.
  - Freshness and policy-matching rules are documented and tested. Missing,
    stale, or mismatched evidence produces pending verification with a next
    action; a matching failed diagnostic produces failed verification.
  - The AW-001 `addons` result cannot verify the `hub-agent-codex` profile or
    its exact model policy. Readiness states can be implemented and tested
    offline before the dedicated execution runner is provisioned in AW-012.

  **Completion note:** Implemented the separate versioned readiness read API and
  typed server-only frontend adapter with all four states, named bootstrap files,
  expected runner labels, and next actions. Added the closed v1 evidence schema
  and a private-only, checksum-pinned no-write producer for the dedicated profile;
  the legacy `addons` diagnostic remains independent. The GitHub reader verifies
  latest run/attempt/job origin, exact policy/revision, 24-hour freshness, artifact
  binding/digest, and bounded archive/schema shape. Updated generated contracts,
  permissions, operator recovery docs, and canonical bootstrap conventions.
  Full `make check` passed (81 frontend tests plus Go/Python/Node/workflow/contract
  checks); Agents/frontend images built and an isolated Agents container passed
  health and readiness parameter validation. No live credentialed diagnostic was
  dispatched; dedicated runner provisioning/live verification remains AW-012,
  and the UI remains AW-007.

- [x] **AW-007 — Build the profile and readiness UI**

  **Domain:** frontend / agents  
  **Depends on:** AW-005, AW-006

  Add a repository-level view that lists profiles and their readiness.

  **Acceptance criteria:**

  - `codex-thorough` shows its role, authority, runner requirements, and
    configuration/runtime state.
  - Loading, empty, error, stale, and success states are explicit.
  - The UI is accessible, responsive, and works in dark mode.

  **Completion note:** Added the `/agents` workdesk tab and repository-card links,
  selected-repository controls, profile policy/runner context, distinct configuration
  and runtime states, safe evidence disclosures, and GitHub source links. The
  server joins matching catalog/readiness snapshots and preserves policy during
  readiness outages. Explicit loading, empty, error, and stale states prevent old
  or mismatched evidence from appearing currently verified. Native GET forms and
  initial rendering work without JavaScript; client navigation streams loading.
  Full `make check` passed (113 frontend tests plus Go/Python/Node/workflow/contract
  checks), and production/Compose frontend builds passed. Controlled Chromium
  checks verified repository switching, refresh/loading, failure recovery, keyboard
  disclosures, freshness expiry, dark mobile layouts at 390px/320px, and no-JavaScript
  selection. No GitHub writes or live credentialed diagnostics were performed.

## Phase 2 — Bootstrap GitHub-native conventions

- [x] **AW-019 — Implement Hub GitHub identity and write authorization**

  **Domain:** identity

  **Depends on:** AW-004

  Implement GitHub sign-in, server-side sessions and token lifecycle, and
  repository-scoped write authorization for Hub bootstrap and assignment
  actions. Use the GitHub App user-to-server identity established by AW-002;
  keep this capability separate from the read-only discovery token.

  **Acceptance criteria:**

  - Sign-in and callbacks bind the verified GitHub identity to a server-side
    session; callback and mutating requests have forgery protection.
  - GitHub credentials remain server-side and cannot enter client bundles,
    browser-accessible storage, responses, logs, or repository artifacts.
  - Token expiration, refresh where supported, revocation, and sign-out have
    defined behavior and useful recovery guidance. Invalid sessions or tokens
    fail closed before a GitHub write.
  - Each write verifies the signed-in user's current repository access and
    permissions required by that action. Assignments require current
    `maintain`/`admin` access and create a comment attributed to that user.
  - Request only the permissions required by bootstrap PRs and assignment
    comments. Read-only discovery credentials cannot authorize mutations.
  - Controlled integration tests cover identity attribution, unauthorized
    repository access, forged requests, and expired/revoked credentials.
    Backend ownership and the public API are documented under the repository's
    Go service/OpenAPI conventions; SvelteKit owns the sign-in interface.

  **Ordering:** The new ID preserves AW-001–AW-018 references. This task
  precedes AW-009/AW-014; GitHub-native execution does not depend on Hub sign-in.

  **Completed (2026-10-01):** Added the independent Identity Go service and
  generated v1 OpenAPI contracts, GitHub App user sign-in with browser-bound
  one-use state/PKCE, encrypted server-side sessions, refresh/revocation, and
  exact-role/current-installation authorization. The `/account` interface and
  server-only auth routes support safe recovery and native forms. Future
  AW-009/AW-014 writes must use Identity's `WithAuthorization` boundary;
  controlled GitHub HTTP tests prove user-attributed comments and access/token
  revocation rejection. Compose and component/operator documentation are updated.
  Full `make check` passed (137 frontend tests plus Go/Python/Node/workflow/contract
  checks); Identity race tests and static analysis passed. Identity/frontend
  container builds and isolated disabled/configured vault smoke checks passed.
  Controlled Chromium checks verified sign-in, refresh, sign-out/revocation
  recovery, outages, keyboard access, dark mobile layouts, and JavaScript-disabled
  forms. No live GitHub sign-in or writes were performed; those require a real
  App/installation configured using the [setup guide](services/identity/README.md).

- [ ] **AW-008 — Define bootstrap PR contents and generator**

  **Domain:** agents  
  **Depends on:** AW-003, AW-006, AW-011, AW-013

  Build a deterministic generator for the GitHub-owned artifacts needed by
  `codex-thorough`: profile catalog, adapter workflow, validation/diagnostic
  workflow, documentation, and any proposed `AGENTS.md` additions.

  **Acceptance criteria:**

  - Generated artifacts are reviewable, repository-local, and contain no
    credentials.
  - Re-running generation is idempotent and presents a clear diff.
  - Generated workflow uses a dedicated runner label and least privilege.
  - Bootstrap output uses the implemented AW-011–AW-013 workflows and AW-006
    diagnostic evidence contract as canonical templates. It installs the
    working GitHub-native flow without placeholder or independently maintained
    execution logic; runner provisioning remains an explicit manual step.
  - Generated files use AW-006’s canonical bootstrap paths and include the
    pinned validator/authorization helpers needed by the working workflows.

- [ ] **AW-009 — Create a bootstrap PR from Hub**

  **Domain:** agents / GitHub integration  
  **Depends on:** AW-008, AW-007, AW-019

  Add the Hub action that creates a branch and draft PR containing the
  generated bootstrap artifacts.

  **Acceptance criteria:**

  - The action explains every GitHub write before it occurs.
  - The PR links back to the selected repository/profile setup and contains a
    manual runner-installation checklist.
  - Hub retains no hidden configuration after the PR is created.
  - Write authority is distinct from the existing read-only discovery token.
  - The action uses AW-019's authenticated identity and repository write
    authorization, and requires reconnecting when that access is unavailable.

- [ ] **AW-010 — Profile editor and setup tutorial**

  **Domain:** frontend / agents  
  **Depends on:** AW-008, AW-009

  Create a guided UI for authoring a profile and producing the corresponding
  bootstrap/configuration PR.

  **Acceptance criteria:**

  - The editor exposes profile role, adapter, model policy, triggers, context,
    authority, validation, image, and pipeline settings.
  - It presents adapter-specific requirements and a runner setup tutorial
    before creating changes.
  - Draft state is browser-local or a GitHub draft PR, never Hub-only durable
    state.

## Phase 3 — Execute one `codex-thorough` assignment

- [x] **AW-011 — Implement the trusted GitHub Actions intake workflow**

  **Domain:** repository workflow / agents  
  **Depends on:** AW-001, AW-002, AW-003

  Implement the repository workflow that receives the chosen GitHub assignment
  event, resolves the pinned profile, validates the actor and authority, and
  dispatches `codex-chatgpt-private-runner`.

  **Acceptance criteria:**

  - The workflow reads current GitHub issue/PR state at execution time.
  - Untrusted actors, unsupported profiles, and malformed requests fail with a
    clear GitHub check/comment.
  - The workflow uses the profile revision recorded in the assignment.
  - Both pinned and current policies validate and permit the assignment.
    Disabled/removed profiles and material policy changes fail closed, and
    current requester/rerun-actor permissions are checked before dispatch.
  - Tests cover duplicate delivery, reruns, concurrent requests, and policy or
    permission revocation. Duplicate delivery cannot create a new assignment;
    retries/reruns resume the same GitHub-native assignment without concurrent
    duplicate work. Distinct comment IDs remain separate assignments.

  **Completed (2026-10-01):** Added `agent-assignment.yml` and independent
  `ops/agent-intake` Go tooling using `go-github` and the canonical AW-003 Python
  validator. Intake verifies live source/requester/rerun access, exact revision
  ancestry, enabled pinned/current profiles, and material policy agreement,
  rechecking source/roles/moving policy before acceptance. Repository/comment IDs
  define assignment identity and workflow concurrency; only the earliest verified
  workflow run dispatches. Reruns preserve one receipt and accepted base; receipt
  publication is reconciled against the exact attempt before dispatch. Safe
  GitHub checks/receipts and a v1 invocation artifact provide the handoff to
  `codex-chatgpt-private-runner`; its hosted gate explicitly reports
  `RUNNER_NOT_READY` until AW-012 implements/verifies the dedicated patch job.
  Controlled HTTP tests cover nine concurrent duplicates producing one dispatch,
  retries, partial receipt publication, forged/truncated provenance, and current
  permission/profile revocation. Full `make check` passed (137 frontend tests,
  Go/Python/Node/workflow/contract checks); final intake checks, nine new Python
  tests, and Go race tests passed. CI and component/architecture documentation
  are updated. No workflow was published/triggered or source executed live.

- [x] **AW-012 — Implement the self-hosted Codex patch job**

  **Domain:** repository workflow / runtime adapter  
  **Depends on:** AW-011, AW-006

  Run Codex in an isolated checkout on the approved runner and emit a patch plus
  structured summary/evidence.
  Provision and verify the dedicated execution runner before any live source
  execution; workflow development and offline tests may proceed independently.

  **Acceptance criteria:**

  - The Codex job has repository-read scope and the least sandbox/tool access
    needed.
  - Subscription authentication stays on the runner and cannot appear in job
    output or repository-controlled files.
  - The job creates a patch artifact rather than pushing directly to GitHub.
  - Failure and cancellation leave useful GitHub-visible evidence.
  - A dedicated restricted host/account is provisioned with the profile's
    `hub-agent-codex` label and approved repository access. The shared `addons`
    spike exception does not authorize source execution.
  - Live verification proves the exact `gpt-6.1-sol`/`high` policy with no
    fallback and publishes matching AW-006 evidence before enabling execution.
  - Isolation checks demonstrate that workload code cannot read host
    authentication or unrelated credentials, inherit ambient Codex configuration,
    or use workload network access. Operator setup/revocation procedures are
    documented; adding a runner label alone does not satisfy this prerequisite.

  **Operator exception (2026-10-02):** The operator explicitly selected the
  existing shared `addons` runner/account for public `r59q/hub-rearranger`.
  This overrides the dedicated host/account and private-target prerequisites
  for this named repository. Workload isolation, current maintainer/default-branch
  guards, exact model/reasoning policy, and origin-verified AW-006 evidence remain
  required before execution is enabled.

  **Completed and verified (2026-10-03):** The read-only execution verifier,
  gated patch job, operator-installed isolated launcher, closed result contract,
  protected-path/patch checks, fixed offline validation, prior-attempt reconciliation,
  and hosted failure/cancellation reporting are published. Full `make check`
  passed, including 137 frontend tests and Go/Python/workflow/contract checks;
  the verifier repair also passed intake checks and Go race tests.

  Origin, archive digest, schema, and freshness checks verified the latest
  exact-policy AW-006 artifact from
  [run 37073496170](https://github.com/r59q/hub-rearranger/actions/runs/37073496170)
  at `205ae9755bba04d534590d6385fc0f7fcd0d5603`. The controlled assignment on
  [issue #5](https://github.com/r59q/hub-rearranger/issues/5#issuecomment-5962748237)
  completed all four jobs in
  [run 37074982572](https://github.com/r59q/hub-rearranger/actions/runs/37074982572).
  Its [patch artifact](https://github.com/r59q/hub-rearranger/actions/runs/37074982572/artifacts/11255798549)
  has archive SHA-256
  `fa88fce0a739ab1dded16caee11a5e0192223eb6a765094a6486a30870bc485b`.
  Independent verification checked assignment/run/attempt, base/profile/policy
  revisions, the result schema, patch and summary digests, and patch application.
  The proposal adds only `docs/agent-patch-review.md` with the exact requested
  contents. It records subscription-backed Codex 0.159.3, effective
  `gpt-6.1-sol`/`high`, verified isolation, and workload networking disabled.
  The fixed offline repository check is explicitly `failed`, as permitted by
  `draft-with-evidence`; workflow success does not claim validation passed.
  Private source workspaces were removed. Execution published no branch or PR.

  Live verification found two setup issues before source execution: reduced
  Actions repository objects omit `default_branch`, so the verifier now reads
  current policy from the repository endpoint; default Python virtual environments
  contain mode-777 symlinks rejected by the workflow guard, so the installed
  environment uses ordinary copies and the exact guard was rechecked. Both failed
  attempts were reconciled before new explicit assignments. The operator guide
  documents this setup. Default-branch changes require fresh AW-006 evidence;
  the operator gate is disabled for this completion update until that evidence
  is renewed. AW-013 remains the separate branch/draft-PR publication task.

- [ ] **AW-013 — Implement the separate branch and draft-PR write job**

  **Domain:** repository workflow / GitHub integration  
  **Depends on:** AW-012

  Apply the patch in a separate job that has branch/PR write authority but no
  Codex authentication.

  **Acceptance criteria:**

  - Branches are unique and attributable to the assignment.
  - Draft PR provenance records the source issue, profile ID/revision,
    authority, and validation evidence.
  - The job posts a concise issue/PR update and GitHub check.
  - It cannot merge or update an unrelated branch.
  - Before applying a patch, the job independently verifies artifact identity
    and digest against the originating assignment/run/attempt, current profile
    policy and authorization, and the expected repository/branch head.
  - Protected-path and patch checks enforce the AW-003 adapter contract,
    including `.github/**`, runtime/profile tooling, instructions, credentials,
    path escapes, unsafe symlinks, and submodule changes. Tests cover forged
    artifacts, protected changes, revoked policy, and stale heads.
  - Duplicate events, reruns, and concurrent attempts cannot create duplicate
    branches or PRs or overwrite another attempt. Tests prove recovery from
    partial publication, such as a branch created before PR creation failed,
    by reconciling verified GitHub artifacts without force-pushing or claiming
    success before publication completes.

  **Implemented locally and verified (2026-10-03):** Added a separate hosted publisher,
  independent artifact/result/current-authorization checks, private cached Git
  patch application using the canonical protected paths, deterministic Git-object
  and branch creation, draft provenance, issue link and honest validation check.
  Receipt intent survives reruns and prevents blind comment/check POST retries;
  recovery verifies actual GitHub objects before claiming completion. Tests cover
  forged artifacts, protected/unsafe patches, revoked roles/policy, stale heads,
  lost responses, partial branch/PR publication, and an overlapping old attempt.
  Full `make check` passed, including 137 frontend tests, 21 intake Python tests,
  Go integration tests, and workflow/profile/contract checks; Go race tests also
  passed. Runner execution remains disabled. Merge/review, updated operator verifier,
  fresh readiness evidence, and controlled live draft-PR/retry verification remain
  required; this task remains unchecked.

- [ ] **AW-014 — Add Hub assignment UI and derived run view**

  **Domain:** frontend / agents  
  **Depends on:** AW-011, AW-013, AW-019

  Let a user assign `codex-thorough` from a Hub issue view by writing the same
  GitHub-native request used outside Hub. Display its GitHub-derived outcome.

  **Acceptance criteria:**

  - Hub does not invoke or poll a runner directly.
  - The assignment UI explains profile authority before the GitHub write.
  - Status, PR, branch, check, and summary are derived from GitHub artifacts.
  - Assignment comments use AW-019's signed-in user identity; missing or revoked
    access prompts reconnection rather than falling back to a bot or read token.

## Phase 4 — Pull-request follow-up

- [ ] **AW-015 — Introduce the pull-requests read domain**

  **Domain:** pull requests  
  **Depends on:** AW-013

  Add a focused PR read model for review threads, assignment provenance,
  branches, and related checks.

  **Acceptance criteria:**

  - The domain remains a GitHub projection and does not duplicate PR state.
  - It exposes enough data to resolve agent PR ownership and show follow-up
    status.

- [ ] **AW-016 — Route one PR review comment through GitHub**

  **Domain:** repository workflow / pull requests  
  **Depends on:** AW-015, AW-011, AW-013

  When a trusted reviewer comments on an agent-created PR, invoke the profile
  recorded by its provenance and update only that agent branch.

  **Acceptance criteria:**

  - The workflow uses the latest PR head, diff, review thread, and checks.
  - A comment on an agent PR does not require the reviewer to repeat model or
    provider information.
  - Ambiguous or human-owned PRs require an explicit new profile assignment.
  - The subsequent commit/check links back to the review comment.

## Phase 5 — Verification and handoff

- [ ] **AW-017 — Repository end-to-end integration test plan**

  **Domain:** testing / documentation  
  **Depends on:** AW-016

  Define and execute a controlled repository test: bootstrap,
  diagnostics, issue assignment from both surfaces, draft PR, and review
  follow-up.

  **Acceptance criteria:**

  - The test does not require live production repositories or expose source
    content in fixtures/logs.
  - Results document expected GitHub artifacts and recovery actions.
  - Subscription usage and runner authentication handling are reviewed.

- [ ] **AW-018 — Document the supported convention and operational guide**

  **Domain:** documentation  
  **Depends on:** AW-017

  Document installation, profile format, bootstrap procedure, runner setup,
  diagnostics, assignment, PR follow-up, credential rotation, and removal of
  Hub.

  **Acceptance criteria:**

  - A maintainer can bootstrap and operate the workflow solely from GitHub.
  - The guide clearly labels advanced subscription-authentication requirements.
  - Documentation covers failure, revocation, and runner replacement.

## Explicitly deferred

- Codex cloud and API-key adapters.
- Copilot, Gemini, and other provider adapters.
- Parallel agent comparisons and cross-agent handoff.
- Automated CI remediation, pipeline analysis, and visual/image workflows.
- Organization-level templates, governance, and fleet views.
- Merge, release, secret, permission, and workflow-configuration authority for
  agents.
