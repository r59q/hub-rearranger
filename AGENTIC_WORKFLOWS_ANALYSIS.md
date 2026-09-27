# GitHub-Native Agent Workflows

> A concise working agreement for Hub Rearranger. This describes intended
> product conventions, not committed implementation.

## The core decision

Hub Rearranger is an optional, focused interface over GitHub-native agent
workflows. It does not orchestrate, queue, route, resume, or own agent runs.

GitHub is the durable source of truth and orchestration substrate:

- issues, pull requests, comments, review threads, checks, workflow runs,
  attachments, branches, and commits are GitHub objects;
- repository configuration and GitHub Actions define profiles and runtime
  routing;
- GitHub events start work, deliver follow-up, and trigger pipeline analysis;
- provider GitHub Apps and GitHub Actions runners perform the actual execution;
- Hub reads this state, offers a better cross-agent view, and writes the same
  GitHub requests a user could make directly in GitHub.

If Hub is removed, the repository's agent workflows continue to work from
GitHub. The GitHub interface is less focused, but no essential control path or
meaning is lost.

## The shared workflow

An agent assignment is a GitHub-visible request to apply a named profile to an
issue or pull request.

```text
Issue or PR assignment
  -> GitHub event
  -> repository GitHub Actions workflow or provider GitHub App
  -> named runtime adapter
  -> dedicated branch, draft PR, checks, and comments
```

Assignments can start from either place:

- **Hub:** the user selects a profile; Hub writes the repository's normal
  GitHub assignment request.
- **GitHub:** the user uses the repository's documented App action, label, or
  profile-named comment convention.

Both produce the same GitHub event and execution path. The request records the
source object, selected profile ID, profile revision, requester, and declared
authority. It never includes credentials.

GitHub is not a universal agent scheduler. The repository supplies the
provider-specific wiring through GitHub Actions workflows or an installed
provider GitHub App.

## Profiles

Every repository owns its profiles. The only profile catalog is:

```text
.github/agent-profiles.yml
```

There are no organization-level or Hub-global profiles. Hub may edit this file
through a branch and pull request, but a maintainer can edit it directly in
GitHub at any time.

A profile is a versioned execution contract, not just a model name. It defines:

| Area | Purpose |
| --- | --- |
| Identity | Stable ID, display name, description, and role. |
| Adapter | Named runtime adapter and its required runner or provider integration. |
| Model policy | Provider-supported model/mode/reasoning request and any permitted fallback. |
| Intake and context | Allowed events and the live GitHub context to collect. |
| Authority | Permitted GitHub writes, sandbox/tool boundaries, branch rule, and draft-PR/merge policy. |
| Validation | Required checks and criteria for ready, blocked, or failed. |
| Continuation | Whether review comments, images, or pipeline events can request follow-up. |
| Presentation | Standard check and comment wording. |

The assignment snapshots the profile revision. Changing a profile later cannot
change the meaning of an existing PR. Material changes in provider, authority,
or model policy create a successor assignment rather than silently mutating
history.

## Runtime adapters

A runtime adapter is repository-owned GitHub Actions or provider-App wiring
that turns a profile into one concrete agent invocation. It receives a GitHub
event, validates the profile and actor, gathers permitted current GitHub
context, invokes one runtime, and writes GitHub-native results.

All adapters should:

- work from live issue/PR comments, diffs, checks, logs, and artifacts rather
  than copied Hub state;
- use an isolated checkout and produce a patch before modifying a branch;
- create or update only their dedicated agent branch and draft PR;
- publish checks/comments with summary, evidence, questions, and failure
  reasons;
- report incompatible configuration visibly instead of choosing another model
  or provider silently.

### Example: `codex-thorough`

`codex-thorough` is a profile. It may use an adapter such as
`codex-chatgpt-private-runner` (a deliberately explicit name) or
`codex-cloud`.

`codex-chatgpt-private-runner` means:

```text
GitHub event
  -> GitHub Actions job on a dedicated self-hosted runner
  -> Codex runs in an isolated checkout using approved authentication
  -> patch artifact
  -> separate GitHub write job
  -> agent branch, draft PR, check, and summary comment
```

It self-hosts the **runner**, not the Codex model. The runner must be separate
from the Hub Docker Compose stack and must not share Hub storage, unrelated
credentials, or unrelated repository mounts.

The safest pattern splits trust zones: the Codex job has repository-read access
and the minimum model credential; a separate job has branch/PR write access but
no model credential. The adapter runs only for trusted repository actors and
never on public, fork-triggered, or shared-runner workflows.

Authentication is adapter-specific. A provider-native cloud adapter can use a
connected subscription-supported provider workflow. A private runner may use a
supported workspace token or advanced persisted account authentication, but
that identity is a sensitive runner secret—not a Hub value, repository file,
or issue comment. API-key automation remains a distinct, explicit option.

## Pull requests, feedback, images, and pipelines

An implementation assignment produces its own branch and draft PR. Its PR body
or check records the source issue, profile ID/revision, authority, and
validation evidence. Parallel agents never share a branch.

Review comments route by PR provenance. A normal comment on an agent-created
PR becomes follow-up for the profile that created that PR; the adapter fetches
the current head, diff, thread, and checks before acting. Ambiguous work or an
agent takeover requires a new explicit profile assignment.

Images attached to issues/PRs and visual-test screenshots remain GitHub
artifacts. Profiles may include them as permitted live context. GitHub Actions
checks, logs, annotations, and artifacts also remain canonical pipeline
evidence. A profile may observe pipelines, analyze failures, or update only
its own PR branch when its authority explicitly allows it.

## Hub's value: bootstrap, diagnostics, and usability

Hub makes the convention easy to adopt without becoming a dependency.

It reads profiles and derives requirements from the repository. It distinguishes:

- **Configuration present:** profile catalog, adapter workflow, triggers,
  permission split, instructions, and documentation exist.
- **Runtime verified:** a no-write GitHub Actions diagnostic confirms runner
  availability, Codex/runtime version, and authentication readiness without
  exposing credentials.

When requirements are missing, Hub explains them and offers **Create bootstrap
PR**. That reviewable GitHub PR can add:

- profile catalog entries;
- GitHub Actions adapter and validation workflows;
- profile instruction/prompt files where appropriate;
- GitHub-native operating and runner-setup documentation;
- an optional proposed `AGENTS.md` addition;
- a smoke-test/diagnostic workflow.

It never adds credentials, copies authentication state, registers a runner, or
silently changes merge protection.

The profile editor is a GitHub-configuration authoring wizard:

1. Name the profile and choose its role.
2. Choose a runtime adapter and view its requirements and limitations.
3. Choose triggers and permitted live context.
4. Set authority, branch/PR behavior, sandbox, validation, image, and pipeline
   policy.
5. Review the generated GitHub changes and required manual steps.
6. Create a branch and pull request containing the profile, workflow, docs,
   and validation changes.
7. Merge it, complete runner setup, and run the GitHub-native diagnostic.

Any editor draft should be browser-local or a GitHub draft PR, never durable
Hub-only configuration.

## Domain map

The existing backend domains are **repositories** and **issues**. They should
remain focused:

| Domain | Status | Responsibility |
| --- | --- | --- |
| Repositories | Exists | Repository discovery, selection, identity, and basic metadata. It does not own agent profile semantics. |
| Issues | Exists | Issue views, relationships, and issue-specific UI context. It asks for available profiles; it does not execute them. |
| Pull requests | Needed | PRs, review threads, agent provenance on PRs, branches, and related checks. This deserves its own domain once PR workflows are introduced. |
| Agents | Needed | Agent profiles, static readiness, bootstrap-PR planning, GitHub assignment requests, and deriving an assignment view from GitHub artifacts. It owns no queue, runner, transcript, or provider credential. |
| Pipelines | Later, if needed | Cross-PR/repository workflow runs, artifacts, and analysis. Initially, related checks can remain part of the pull-request view. |

The **Agents** domain is intentionally not an orchestration domain. Its job is
to understand and help install repository conventions that GitHub orchestrates.
It can validate `.github/agent-profiles.yml`, explain missing adapter files,
prepare a bootstrap PR, and write a GitHub assignment request; the GitHub
Action or provider App performs the work after that.

Runtime adapters and self-hosted runners are not Hub services. They are
repository GitHub Actions/provider-App integrations and external execution
infrastructure. GitHub credentials and provider authentication remain in their
respective GitHub settings, Actions secrets, or runner secret stores.

## Organization extension

An organization can standardize agent workflows without making Hub or a hidden
global profile catalog the source of truth. Keep the **effective profile** in
each repository's `.github/agent-profiles.yml`; use organization-owned GitHub
resources for shared infrastructure and governed reuse.

| Organization concern | GitHub-native mechanism |
| --- | --- |
| Approved adapter implementations | A versioned organization repository of reusable GitHub Actions workflows and adapter documentation. |
| Profile starting points | Versioned templates that Hub or maintainers copy into each repository as local profiles. |
| Safe updates | Per-repository pull requests that bump a pinned reusable-workflow/template revision; never an invisible central behavior change. |
| Runner isolation and capacity | GitHub Actions runner groups restricted to selected repositories. |
| Provider installations and credentials | Organization GitHub App installation, Actions environments/secrets, and runner secret management. |
| Governance | Rulesets, required checks, environment protections, and CODEOWNERS protection for `.github` profile/workflow files. |
| Fleet visibility | GitHub-native checks and a Hub view derived by reading each repository; Hub owns no fleet state. |

This is a **central platform, local contract** model. The organization can
publish a recommended `codex-thorough` template and a tested Codex adapter, but
each repository opts in by committing its own profile that pins the adopted
version. It may add local constraints, such as a narrower path scope or stricter
validation. Removing Hub does not affect that contract: GitHub Actions still
resolves the repository profile and its pinned shared workflow.

The organization may use a centrally referenced profile definition instead of
copying it, but only if the reference is pinned to an immutable revision and
the local repository records its authority/override policy. Copying the small
profile contract locally is simpler, more self-contained, and the recommended
default.

## Safety and minimum outcome

Provider credentials and GitHub authority stay separate. The least useful
authority is the default: read context, create a dedicated branch, and open a
draft PR. Merging, releases, secrets, permission changes, and workflow changes
always need separate explicit approval.

Every successful code-change assignment should leave GitHub with:

- branch and commits;
- draft PR;
- concise summary and limitations;
- validation evidence;
- a trace from request and review feedback to the resulting updates.

## Initial vertical slice

1. Select a repository in Hub.
2. Detect or bootstrap `.github/agent-profiles.yml` and one GitHub Actions
   adapter: `codex-chatgpt-private-runner`.
3. Support `codex-thorough` using the authenticated, subscription-backed Codex
   CLI on a dedicated private GitHub Actions self-hosted runner.
4. Show configuration-present versus runtime-verified diagnostics, including
   runner availability, Codex installation, and authentication readiness.
5. Assign an issue from Hub or GitHub and create the same GitHub request.
6. Produce a branch, draft PR, and GitHub check through the read-only
   Codex-job / separate-write-job trust split.
7. Route one PR review comment back through the GitHub workflow.

This slice intentionally excludes Codex cloud, API-key automation, other
providers, parallel-agent comparison, and automated pipeline remediation. It
uses a trusted private runner only; subscription authentication is treated as a
sensitive runner secret and is never stored in Hub, GitHub repository files,
or issue/PR content.

## Open decisions

- Which GitHub-native assignment surface should be the primary convention:
  App action, label, or comment command?
- What is the smallest, secure profile schema and adapter interface?
- What is the safest supported way to provision, rotate, and health-check the
  subscription-backed Codex authentication on the private runner?
- How are profile-triggered pipeline remediation and image access approved?
- How should a GitHub-native diagnostic safely verify authentication readiness?
