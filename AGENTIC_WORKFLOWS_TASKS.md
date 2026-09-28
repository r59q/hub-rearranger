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

- [ ] **AW-001 — Self-hosted runner and subscription-authentication spike**

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

  **Progress (2026-09-28):** Prepared the maintainer-gated diagnostic,
  checksum-pinned host probe, offline tests, and
  [operating guide](ops/private-runner/README.md) for runner `addons`. Remains
  unchecked pending a successful live Actions run and operator review of the
  procedure. The operator approved using this public repository after a live
  GitHub role check for both original and rerun actors.
  SSH inspection confirmed the Hub runner is active, Codex 0.158.0 is on its
  service PATH, and cached ChatGPT login is present. A trusted SSH invocation
  successfully completed the fixed live subscription request. The operator
  explicitly relaxed isolation for this spike: the shared `r59q` account is a
  temporary exception, not a blocker. The trusted-actor and no-fork
  restrictions remain. The probe is installed under the runner account and
  passed again from its installed path; all seven offline tests and workflow
  lint pass. See the operating guide for evidence and installation.

- [ ] **AW-002 — Lock the GitHub-native assignment convention**

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

- [ ] **AW-003 — Define the minimal profile schema and adapter contract**

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

## Phase 1 — Add the Agents domain read path

- [ ] **AW-004 — Scaffold the Agents service and public contract**

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

- [ ] **AW-005 — Read and validate repository-local profiles**

  **Domain:** agents  
  **Depends on:** AW-004

  Read `.github/agent-profiles.yml` from the selected repository and expose
  parsed profiles, profile revisions, and validation results.

  **Acceptance criteria:**

  - Missing, malformed, and unsupported profile files produce actionable
    states rather than generic failures.
  - Tests cover valid `codex-thorough`, invalid schema, and missing-file cases.
  - No profile data is persisted as a competing Hub source of truth.

- [ ] **AW-006 — Derive readiness diagnostics**

  **Domain:** agents  
  **Depends on:** AW-005, AW-001

  Derive “configuration present” from repository files and GitHub workflow
  metadata. Read the no-write GitHub diagnostic result as “runtime verified.”

  **Acceptance criteria:**

  - The UI/API clearly distinguishes missing configuration, pending runtime
    verification, verified runtime, and failed verification.
  - The diagnosis names missing files, expected runner label, and next action.
  - Authentication is never exposed; only safe readiness status is shown.

- [ ] **AW-007 — Build the profile and readiness UI**

  **Domain:** frontend / agents  
  **Depends on:** AW-005, AW-006

  Add a repository-level view that lists profiles and their readiness.

  **Acceptance criteria:**

  - `codex-thorough` shows its role, authority, runner requirements, and
    configuration/runtime state.
  - Loading, empty, error, stale, and success states are explicit.
  - The UI is accessible, responsive, and works in dark mode.

## Phase 2 — Bootstrap GitHub-native conventions

- [ ] **AW-008 — Define bootstrap PR contents and generator**

  **Domain:** agents  
  **Depends on:** AW-003

  Build a deterministic generator for the GitHub-owned artifacts needed by
  `codex-thorough`: profile catalog, adapter workflow, validation/diagnostic
  workflow, documentation, and any proposed `AGENTS.md` additions.

  **Acceptance criteria:**

  - Generated artifacts are reviewable, repository-local, and contain no
    credentials.
  - Re-running generation is idempotent and presents a clear diff.
  - Generated workflow uses a dedicated runner label and least privilege.

- [ ] **AW-009 — Create a bootstrap PR from Hub**

  **Domain:** agents / GitHub integration  
  **Depends on:** AW-008, AW-007

  Add the Hub action that creates a branch and draft PR containing the
  generated bootstrap artifacts.

  **Acceptance criteria:**

  - The action explains every GitHub write before it occurs.
  - The PR links back to the selected repository/profile setup and contains a
    manual runner-installation checklist.
  - Hub retains no hidden configuration after the PR is created.
  - Write authority is distinct from the existing read-only discovery token.

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

- [ ] **AW-011 — Implement the trusted GitHub Actions intake workflow**

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

- [ ] **AW-012 — Implement the self-hosted Codex patch job**

  **Domain:** repository workflow / runtime adapter  
  **Depends on:** AW-011

  Run Codex in an isolated checkout on the approved runner and emit a patch plus
  structured summary/evidence.

  **Acceptance criteria:**

  - The Codex job has repository-read scope and the least sandbox/tool access
    needed.
  - Subscription authentication stays on the runner and cannot appear in job
    output or repository-controlled files.
  - The job creates a patch artifact rather than pushing directly to GitHub.
  - Failure and cancellation leave useful GitHub-visible evidence.

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

- [ ] **AW-014 — Add Hub assignment UI and derived run view**

  **Domain:** frontend / agents  
  **Depends on:** AW-011, AW-013

  Let a user assign `codex-thorough` from a Hub issue view by writing the same
  GitHub-native request used outside Hub. Display its GitHub-derived outcome.

  **Acceptance criteria:**

  - Hub does not invoke or poll a runner directly.
  - The assignment UI explains profile authority before the GitHub write.
  - Status, PR, branch, check, and summary are derived from GitHub artifacts.

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
