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
- **GitHub:** the user posts the documented profile assignment comment on an
  issue or, when the profile permits it, a pull request.

Both produce the same GitHub event and execution path. The request records the
source object, selected profile ID, profile revision, requester, and declared
authority. It never includes credentials.

## AW-002 assignment convention (v1)

The initial trigger is a newly created, top-level GitHub issue comment. It is
usable from GitHub without installing a separate assignment App. Labels cannot
carry a pinned profile revision or an explicit authority request. GitHub's
[`issue_comment.created` event](https://docs.github.com/en/actions/reference/workflows-and-actions/events-that-trigger-workflows#issue_comment)
also covers PR conversation comments; the intake must distinguish the parent
object. Edited comments do not create or change an assignment. Review-thread
follow-up is a separate event and policy (AW-016).

Post this **single line** on the source issue:

```text
/agent assign codex-thorough@0123456789abcdef0123456789abcdef01234567 authority=branch-draft-pr
```

Replace the example SHA with a full 40-character Git commit SHA from the
default-branch history of `.github/agent-profiles.yml`. A maintainer can open
that file in GitHub, choose **History**, and copy the commit SHA for the
profile revision they reviewed. The ID is the exact profile ID in that file.
The first version accepts one profile ID and one authority value; it rejects
extra text, duplicate fields, abbreviations, and ambiguous revisions. No
credential, prompt, or free-form command belongs in the assignment comment.

| Field | Canonical source and meaning |
| --- | --- |
| Source | GitHub repository and issue/PR containing the comment, read from GitHub's API; never a user-supplied URL. |
| Request | Immutable GitHub comment ID and URL; a new comment is a new request. |
| Profile ID | `codex-thorough` in the command; match the pinned catalog entry exactly. |
| Profile revision | Full default-branch commit SHA in the command. The workflow reads the catalog at that SHA, not at the moving branch tip. |
| Requester | GitHub comment author's account ID and login, fetched at intake; never a claimed name in the body. |
| Authority | `branch-draft-pr` in the command: read the source, propose changes on a dedicated agent branch, open a draft PR, and publish checks/summary comments. It grants no merge, release, secret, permission, or workflow-configuration authority. |

For v1, `codex-thorough` accepts an **issue** as the source. A command on a PR
is parsed as the same convention but rejected unless that profile explicitly
allows a fresh PR assignment. A normal review comment on an agent PR is not a
new assignment; AW-016 will route it through the PR's existing provenance.

The intake workflow runs its authorization step on a GitHub-hosted runner. It
fetches the current comment and parent object and checks that the comment is
still present, unedited, on an open object in the approved repository. It checks
the comment author and any rerun actor for current `maintain` or `admin` access,
as in AW-001. It verifies that the SHA is a default-branch ancestor containing
the profile, and that both the pinned profile and current default-branch policy
permit the requested authority. A changed or revoked policy fails closed; an
old revision does not restore removed authority or an obsolete adapter/model
policy. The write job must reject changes to workflows, profiles, secrets, or
other protected configuration under `branch-draft-pr`. The comment body, issue
contents, and PR contents are untrusted input and must not become shell code.
Only accepted requests reach the credentialed runner. Repeated delivery or
rerun of the same comment ID must not create another assignment or branch.

The workflow posts a GitHub-visible acceptance or rejection on the source
object, linking the exact request comment. An acceptance names the source,
profile ID and revision, requester, authority, and workflow run. Rejections
give a safe reason and a next action. The acceptance is a receipt, while the
request comment and GitHub identity are the provenance; a reply cannot grant
authority by itself.

Hub must create the *same* comment on behalf of the signed-in user with a
GitHub App [user-to-server token](https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/authenticating-with-a-github-app-on-behalf-of-a-user).
GitHub attributes such comments to that user, so the workflow can use the same
current-role check as for GitHub-authored comments. A bot-authored
installation-token comment is not a user assignment.
Hub's existing read-only discovery token cannot submit assignments. This
user-scoped write capability is implemented by AW-019 and consumed by AW-014;
it is not part of this convention task.

The resulting draft PR body contains a visible **Agent assignment** section
with links and these same fields, plus this machine-readable block generated
from verified GitHub data (example values only):

```text
<!-- agent-assignment:v1
{"source":"https://github.com/OWNER/REPO/issues/123","request_comment_id":456,"request":"https://github.com/OWNER/REPO/issues/123#issuecomment-456","profile_id":"codex-thorough","profile_revision":"0123456789abcdef0123456789abcdef01234567","requester_id":789,"requester":"LOGIN","authority":"branch-draft-pr","run":"https://github.com/OWNER/REPO/actions/runs/321"}
-->
```

The PR's dedicated branch and check link back to the same request ID. Future
follow-up reads this provenance to identify the owning profile, then verifies
the linked request, run, branch, and current GitHub authorization. An editable
PR body alone is never proof of ownership or authority. Material changes to
adapter, model policy, or authority require a new assignment rather than
changing the meaning of this PR.

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

## AW-003 profile schema and adapter contract (v1)

The repository now declares `codex-thorough` in
[`.github/agent-profiles.yml`](.github/agent-profiles.yml). Its closed
[JSON Schema](ops/agent-profiles/schema.v1.json) and
[adapter contract](ops/agent-profiles/README.md#adapter-contract-v1) are the
normative v1 definitions. Offline validation uses pinned PyYAML and
`jsonschema`; future consumers use the same schema rather than maintaining
independent field definitions. Catalog validation does not establish runtime
readiness or install assignment execution.

V1 permits only `codex-chatgpt-private-runner`, issue-comment assignment,
`branch-draft-pr` authority, a `workspace-write` sandbox with workload network
disabled, named validation checks, and optional trusted PR review continuation.
Credentials, command strings, arbitrary CLI/provider/workflow overrides,
automatic fallback, images, and pipeline remediation are excluded. The first
profile requests `gpt-6.1-sol` with `high` reasoning; that exact policy still
needs verification on the runner. The dedicated `hub-agent-codex` label does
not extend the shared-runner diagnostic exception.

`schema_version` and `adapter.contract_version` independently version these
contracts. Profile revisions remain full default-branch Git SHAs from AW-002,
never a mutable catalog field. Pinned and current entries must both validate,
exist, and be enabled. Every execution-policy field must match current policy
(context/check lists compare as sets); only display text may change without a
new assignment. Recheck before continuation and patch application so a pinned
revision cannot restore removed authority or obsolete model/adapter policy.

The adapter boundary specifies verified GitHub identity/source/revision/head
metadata, current permitted context, and a patch plus structured outcome and
validation evidence. The credentialed patch job cannot write to GitHub. A
separate write job independently verifies policy, provenance, artifact identity,
protected paths, and head state before publishing a dedicated branch/draft PR.
Malformed profiles produce sorted, safe diagnostics without echoing source
values, profile IDs, arbitrary keys, or parser exceptions. Bounded YAML parsing
rejects duplicate keys, tags, anchors, aliases, and merge keys.

## AW-004 Agents service boundary

[`services/agents`](services/agents/README.md) is an independent Go service
with API, domain, and infrastructure layers. Its v1
[OpenAPI contract](services/agents/api/openapi.yaml) initially exposes process
liveness and read-only assignment-convention metadata. It does not read
repository profiles until AW-005 or derive readiness until AW-006; catalog
validation and liveness must never be presented as runtime verification.

The API generates Go transport DTOs/routes with pinned `oapi-codegen` and
frontend types/enum values with `openapi-typescript`. The server-only SvelteKit
adapter wraps `openapi-fetch`, validates responses, and uses `AGENTS_API_URL`
(loopback port 8082 by default). Domain models remain independent of generated
transport types. Contract checks compare regeneration without editing files.

AW-004 initially ran the Agents service in Compose without GitHub/provider credentials or storage.
Its convention endpoint states the current actor-role and revision requirements
but does not authorize or submit assignments. The frontend has no startup
dependency on Agents, preserving existing views during an Agents outage.
Repository-wide checks include its module, adapter tests, and contract drift
checks; the same suite runs on GitHub-hosted runners without Codex login.

## AW-005 repository profile reads

The Agents v1 API now reads `.github/agent-profiles.yml` through `go-github`
from the selected repository, resolving its current default branch to an exact
commit before fetching the file. The response exposes that commit as the catalog
and each profile's revision; the file blob SHA is never an assignment revision.
Reads fetch fresh GitHub data, send `Cache-Control: no-store`, and retain no
catalog cache or durable profile state. A later execution intake still verifies
revision ancestry, current policy, and actor authorization independently.

The canonical AW-003 Draft 2020-12 schema is exported into checked-in embedded
Go and server-only frontend snapshots and generated `Catalog*` OpenAPI field
shapes by `ops/agent-profiles/export_contract.py`. Contract drift checks verify
these exports and generated DTOs. Full schema validation uses `jsonschema/v6`
and Ajv; the OpenAPI 3.0 projection describes field shapes but omits conditional
and regex semantics. Go's YAML boundary enforces the bounded subset, including
explicit lowercase booleans and rejection of duplicate keys/tags/anchors/aliases.
The service needs no Python runtime.

Missing, invalid, and unsupported catalogs return distinct actionable states
with safe sorted diagnostics and no partial profiles. Valid disabled profiles
remain visible. Validation results describe catalog configuration only, never
runtime readiness. GitHub access, repository absence, rate limits, and upstream
failures have distinct safe transport errors. Repository source/parser/API
exceptions and unknown diagnostic keys/profile IDs are excluded from logs and
error responses.

Compose passes the server-side read token to Agents; private profile reads need
read-only Metadata and Contents permissions. Public catalogs permit unauthenticated
GitHub access. The service remains on loopback/private network with no caller
authentication, like the other local read services; a public multi-user deployment
must add caller authorization before exposing private data. Provider credentials
and runner authentication remain outside Hub.

## AW-006 profile readiness diagnostics

The separate read-only readiness API derives configuration presence from
commit-pinned bootstrap files and active Actions workflow metadata. Its four
profile states are `configuration_missing`, `verification_pending`,
`runtime_verified`, and `verification_failed`, with expected runner labels,
missing file paths, and explicit next actions. Profile validation remains separate;
invalid catalogs expose no profile rows, and readiness never authorizes work.

The canonical bootstrap paths are `AGENTS.md`, `docs/agent-workflows.md`,
`.github/workflows/agent-assignment.yml`, and
`.github/workflows/agent-profile-diagnostic.yml`, alongside the profile catalog.
Metadata checks require assignment `issue_comment.created` intake with
`authorize`/`patch`/`publish` jobs, a manual diagnostic with
`authorize`/`diagnostic` jobs, and static dedicated runner labels. They establish
configuration presence, not workflow security or execution isolation. AW-008
must use these paths and the implemented execution templates and supporting
helpers; no placeholder assignment workflow is added by AW-006.

The new private-only fixed diagnostic probes `codex-thorough` on
`hub-agent-codex` with exact `gpt-6.1-sol`/`high` policy, checks CLI session headers
and a live no-write marker, rejects warning/reroute/ambiguous policy output, and
publishes only a closed versioned evidence record. The credentialed job executes
checksum-pinned operator-installed scripts without checkout or GitHub permissions;
a hosted job uploads its immutable attempt artifact. Legacy AW-001 `addons`
results remain independent and cannot verify a profile's exact runtime policy.

Origin validation uses the registered workflow, latest default-branch run,
exact-attempt authorization/diagnostic jobs, an assigned dedicated runner,
artifact run/repository/head binding, and a bounded SHA-256-verified archive.
Evidence must match the current full commit/profile/runner/model policy, fall
within job timestamps, and be no older than 24 hours. A newer pending attempt
supersedes older successes. Missing, expired, stale, malformed, or mismatched
records remain pending; a fresh matching failed record reports failed verification.
No credential, transcript, raw CLI/GitHub error, or untrusted artifact text reaches
Hub responses or logs. The contract exporter/checks cover the canonical AW-006
schema, Go/frontend snapshots, and `RuntimeEvidence` OpenAPI shapes.

Offline producer, domain, controlled GitHub/storage boundary, API-contract, and
server-only frontend tests verify these rules without live authentication.
Readiness adds Actions read permission to the server-side read token. The API
stays private/loopback with the existing local access boundary; runner provisioning
and live verification remain AW-012, and the repository profile/readiness view
is described below. See [the evidence/operator guide](ops/private-runner/READINESS.md).

## AW-007 profile and readiness view

The `/agents` workdesk tab lists profiles for a selected repository and exposes
role, authority, model/reasoning policy, dedicated runner requirements, configuration
diagnostics, and runtime status together. Native disclosures show execution/review
policy and safe diagnostic evidence; pinned catalog/commit and workflow/run links
preserve access to GitHub. Repository cards link directly to this view.

The server loads catalogs and readiness concurrently and maps validated transport
data into frontend presentation models. Reads must describe the same repository,
branch, revision, profile set, runner, and requested model policy before runtime
proof is shown. A readiness-only failure keeps policy visible with unknown status;
catalog errors never expose partial profile rows. Missing, invalid, unsupported,
empty, loading, error, and success states remain distinct.

The browser marks a view stale after five minutes or evidence expiry (24 hours),
on a snapshot mismatch, or after a failed refresh. Previous evidence cannot keep
a current verification label while refreshing or stale. Initial server rendering
and GET forms work without JavaScript; client navigation streams explicit loading.
The view follows system dark mode and supports keyboard controls and narrow touch
layouts. It reads GitHub state through the existing services and performs no writes
or runner actions. Live dedicated-runner verification still belongs to AW-012.

## AW-019 Hub identity and write authorization

The independent [Identity service](services/identity/README.md) owns GitHub App
user sign-in, encrypted server-side sessions, token refresh/revocation, and live
repository write authorization. The `/account` interface uses server-only
SvelteKit adapters; cookies contain opaque identifiers and credentials stay inside
Identity. Account availability does not block repository or agent read views.
Sign-in uses one-use browser-bound state and PKCE; authenticated mutations require
the exact public Origin and session anti-forgery nonce.

Each authorized operation rechecks the actual GitHub user, current repository role,
configured App installation, repository selection, and required permissions.
Assignments require `maintain`/`admin` and Issues write; bootstrap requires
`write`/`maintain`/`admin` and Contents, Pull requests, and Workflows write.
The discovery token cannot authorize these operations. Revoked/expired credentials
fail closed, transient outages give safe recovery guidance, and sign-out removes
local authority before remote revocation.

AW-009/AW-014 must execute their user-attributed writes through Identity's
`WithAuthorization` boundary, adding versioned mutation APIs with those features.
The public authorization preflight is not a reusable grant and never exports a
token. Other domains retain their business payloads and rules. Controlled GitHub
HTTP tests prove comment attribution and current-access rejection; live App setup
and real GitHub verification still require operator configuration.

## AW-011 trusted assignment intake

The [GitHub-hosted assignment workflow](.github/workflows/agent-assignment.yml)
uses the independent [intake tooling](ops/agent-intake/README.md) to re-fetch the
source comment/open issue, verify current requester/original/rerun roles, resolve
revision ancestry, and validate pinned/current catalogs with the canonical AW-003
parser/schema. Every material execution field must match current policy; source,
role, and moving-branch policy checks run again before acceptance. No comment body
becomes shell code or a provider instruction during intake.

Assignment IDs and workflow concurrency use immutable repository/comment IDs.
Only the earliest verified workflow run for a comment may dispatch; duplicate
deliveries point to that original run. Reruns update one GitHub Actions bot receipt,
retain its accepted base SHA, and recheck current policy rather than trusting prior
acceptance. Ambiguous receipt POSTs are reconciled before dispatch. Missing,
truncated, or inconsistent GitHub run/receipt provenance fails closed.

The versioned invocation artifact carries verified identity/source/profile/base/run
metadata and execution policy, excluding display prose, credentials, source bodies,
and raw errors. Rejections have fixed GitHub check/summary guidance. Acceptance
comments link to the exact request/run and explicitly report `RUNNER_NOT_READY`.
The dispatcher currently runs only an execution-disabled hosted gate. AW-012 must
add and verify the dedicated patch job, reauthorize artifact origin/current policy,
and reconcile execution before retrying; AW-013 independently reconciles branches
and PRs. Receipts and invocation metadata never grant authority. No source execution
or self-hosted runner is enabled by AW-011, and the workflow is not yet published
to a live repository by this local implementation.

## AW-012 runtime and AW-013 publication

AW-012 now has published, origin-verified exact-policy runtime evidence and an
independently verified live patch proposal. The operator's 2026-10-02 approval
explicitly extends the shared `addons` account/runner exception to source execution
for public `r59q/hub-rearranger` only. Workload authentication/configuration access
and networking remain denied. Current readiness is tied to the exact default-branch
revision; historical completion never grants a future execution or publication.

AW-013 adds a separate GitHub-hosted publisher with job-scoped
Contents/Pull requests/Issues/Checks write and Actions read. It has no Codex
credential and executes no proposal source. The same repository/comment
concurrency group covers intake, dispatch, execution, publication, and reporting.
It independently verifies actual artifact origin/archive digest, original
invocation/result/schema/patch/summary identity, current source/roles/policy, and
expected base/head. A private Git index applies the patch using the canonical
AW-012 source limits/protected paths; the original full Git tree preserves
untouched protected files. `go-github` creates immutable Git objects and only
creates a deterministic assignment ref. It never updates a ref, force-pushes,
merges, or edits an unrelated PR.

Recovery reconstructs the same commit and verifies GitHub's branch, bot-owned
open draft PR/provenance, issue comment, and app-owned validation check before
claiming publication. The deterministic commit message omits its final newline
to match GitHub's Git API response; the message, parents, tree, and author metadata
are still compared exactly. Live verification found that the original final
newline caused rejection after immutable commit creation and before any ref/PR.
Ordinary publication failures retain an allowlisted reason code in their status
artifact; generic fallback evidence is reserved for missing command output.
A missing branch/PR can resume; altered/closed/moved or
ambiguous objects fail closed. Because comment/check POSTs have no idempotency
key, the verified intake receipt carries optional publication intent before
those writes. Intake preserves it on rerun. Started intent with a missing object
requires operator reconciliation; intent and receipts never grant authority.
Failed/unavailable offline validation remains explicit and produces a neutral
proposal check, not a claim that repository validation passed.

GitHub replaces a `GITHUB_TOKEN` check's supplied details URL with its own check
page. Reconciliation accepts the original producer URL or the exact repository
and check-ID page while retaining exact app, assignment, head, conclusion, and
summary verification. Tests reproduce the observed response and reject forged
hosts, repositories, and check IDs.

The implementation has controlled HTTP/TLS and real Git/schema tests. Live
assignment 37116695777 completed publication of draft PR #12 with the exact
document patch and matching provenance. Its full rerun lost the original
artifacts and correctly refused recovery without duplicate execution or writes.
This matches [reported GitHub support behavior](https://github.com/orgs/community/discussions/17854):
full reruns replace artifacts; job-specific reruns retain them. Recovery must
use the individual `authorize` job and dependent jobs, retaining earlier evidence.
Unique artifact names, retention settings, and downloaded ZIPs cannot restore
verified origin after deletion. Live [assignment 37117465286](https://github.com/r59q/hub-rearranger/actions/runs/37117465286/attempts/1)
completed publication of draft PR #13. Its [job-specific retry](https://github.com/r59q/hub-rearranger/actions/runs/37117465286/attempts/2)
reauthorized, skipped source execution, preserved all four original artifact
identities/digests/origins, and reconciled the unchanged PR, branch head, issue
update, and validation check without duplicates. AW-013 live verification is
complete. The neutral check explicitly preserves failed offline repository
validation under `draft-with-evidence`. Native Actions pull-request creation must
be enabled in repository settings; no extra long-lived publication credential is
introduced. Readiness must be renewed after default-branch revision changes.

## AW-008 deterministic bootstrap packaging

The Agents domain owns the pure bootstrap file plan and managed instruction block;
`services/agents/cmd/bootstrap` exposes them as an offline local command. This
task introduces no public service endpoint, GitHub write, frontend flow or new
Compose dependency. AW-009 will consume bootstrap planning when implementing its
separately authorized draft-PR action through Identity.

Canonical templates are read directly from a reviewed Hub Rearranger checkout:
the implemented AW-011–AW-013 workflows and Go/Python helpers, AW-003 validator,
AW-006 evidence contract, operator scripts, component docs/ignore rules and the
portable `agent-bootstrap-checks.yml` workflow. There is no copied execution
template tree or generated source snapshot to maintain. Source helper content,
dependency pins, concurrency, protected paths, artifact origin checks, recovery
rules and permission boundaries are retained. Only workflow expressions for
the exact upstream public-runner exception are narrowed to private repositories;
that operator exception does not transfer to installations. Unknown exception
template shapes fail closed. Required readiness paths include the new canonical
`docs/agent-workflows.md` installation/convention guide.

The versioned JSON file plan sorts paths, carries proposed content and SHA-256,
and classifies creations, updates, unchanged files and conflicts. Git renders
an applicable diff with external diff/text conversion disabled. The generator
reads only packaged target paths, rejects symlink/binary/oversized inputs, and
exports only to a new staging directory outside both checkouts. A matching
profile catalog stays byte-for-byte unchanged; adding the missing profile
preserves other entries and comments, with explicit catalog formatting changes.
Existing different/disabled policy and malformed catalogs require review, never
silent replacement or enablement. Instructions outside the one managed marker
pair remain intact; ambiguous markers and oversized merged files fail closed.
Repeated generation against the applied output yields an empty diff.

Runner registration, subscription authentication, pinned operator installation,
source-free isolation/exact-model preflight, fresh origin-verified AW-006 evidence
and both execution gates remain manual. The installation owns its root `make
check` validation and offline dependencies; bootstrap does not replace its
Makefile or claim that a published draft passed validation. GitHub-native issue
assignment/publication works independently of Hub; PR review continuation remains
AW-016. Historical upstream operator approvals in component documentation are
context, never installation authorization.

`make bootstrap-check` exports into a disposable directory, verifies repeat JSON
statuses/diff, and runs the exported installation's canonical offline workflow
checks without Hub or the upstream source tree. It reuses only the pinned local
development environment for testing; exported files contain no virtualenv,
manifest, credentials or authentication. The check is included in `make check`,
so adding a helper dependency or changing workflow pins must continue to produce
an independently buildable installation. Credentialed diagnostics and source
execution are excluded from this validation.

AW-008 completed on 2026-10-03. Full `make check` passed, including the generator
behavior/CLI tests, 137 frontend tests and the exported 93-file installation's
Go/Python/Node/schema/workflow checks. Repeat generation produced only unchanged
files and an empty diff. No GitHub writes, runner provisioning or credentialed
diagnostics/execution were needed to complete bootstrap generation.

## Backlog refinements after AW-005 (2026-10-01)

The post-AW-005 recommendation made AW-006 the next implementation task,
now completed in the section above. Its scope included defining a versioned
profile-specific diagnostic evidence contract and updating the no-write
producer, so the reader can verify repository/profile revision, runner
requirements, exact model/reasoning policy, CLI version, originating workflow
run/attempt, timestamp, and safe outcome. Evidence must be matched to trusted
GitHub workflow/job metadata. Missing, stale, or mismatched evidence means
pending verification; matching failed evidence means failed verification.
Freshness and policy-matching rules must be documented and tested. The AW-001
`addons` success cannot establish readiness for `hub-agent-codex` or the exact
profile model policy. Implementing these states needs no live execution runner.

Dedicated execution-runner provisioning and verification belong to AW-012,
with AW-006 as a dependency. Before live source execution, the restricted
host/account, approved repository access, required label, exact model policy,
and workload isolation must be demonstrated. The temporary shared-account
diagnostic exception remains limited to AW-001; a label alone proves neither
isolation nor authorization.

AW-019 is the single added task, owned by the identity domain. It establishes
Hub GitHub sign-in, server-side sessions and token lifecycle, and current
repository write authorization. AW-009 and AW-014 consume that capability;
assignment comments use the authenticated user's GitHub App user-to-server
identity from AW-002. The existing discovery token remains read-only. Public
API and component ownership are documented during implementation without
putting credential handling into SvelteKit UI code or adding an agent runtime.

AW-011 and AW-013 explicitly test duplicate delivery, reruns, concurrent
requests, and revocation. Retries reuse assignment identity and reconcile
GitHub artifacts after partial publication. AW-013 also independently verifies
artifact provenance/digest, current policy/authorization, protected paths,
and expected heads before applying a patch. These are acceptance criteria for
the existing workflows, rather than additional implementation tasks.

AW-008 depends on the AW-006 evidence contract and implemented AW-011–AW-013
workflows. Its generator packages canonical working templates rather than
maintaining a second execution implementation or installing placeholders.
Existing task IDs remain stable; dependencies determine execution order even
when phase headings or task numbers appear earlier. Hub authorization is not
a prerequisite for operating the GitHub-native workflow directly from GitHub.

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

The existing backend domains are **repositories**, **issues**, **agents**, and
**identity**. They should remain focused:

| Domain | Status | Responsibility |
| --- | --- | --- |
| Repositories | Exists | Repository discovery, selection, and basic metadata. It does not own user identity or agent profile semantics. |
| Identity | Exists (AW-019) | GitHub App user sign-in, encrypted sessions, token lifecycle, and live user-attributed write authorization; no agent execution or discovery token. |
| Issues | Exists | Issue views, relationships, and issue-specific UI context. It asks for available profiles; it does not execute them. |
| Pull requests | Needed | PRs, review threads, agent provenance on PRs, branches, and related checks. This deserves its own domain once PR workflows are introduced. |
| Agents | Profile/readiness reads (AW-005/AW-006) | Convention, validated repository profiles, and derived readiness API; bootstrap-PR planning and GitHub assignment requests follow in later tasks. It owns no queue, runner, transcript, or provider credential. |
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

- How are profile-triggered pipeline remediation and image access approved?

## AW-001 runtime spike decisions

The prepared diagnostic uses advanced persisted ChatGPT account authentication
under the runner service account. Credentials remain on its protected
disk and are never transported through Hub or GitHub. The selected runner is
`addons`; the workflow schedules with its default `self-hosted` and `linux`
labels, then checks `RUNNER_NAME=addons` before invoking Codex. Labels do not
restrict repository access: registration or a restricted runner group does that.

The diagnostic permits only the approved repository's default branch and
current `maintain` or `admin` actors. A separate GitHub-hosted job checks the
GitHub role of both the original and rerun actor before any job reaches the
credentialed runner. It uses only metadata/read access and fails closed when
GitHub cannot verify the role. The Codex job requests no GitHub permissions and
performs no checkout. An operator-installed, checksum-pinned probe checks the
CLI, cached ChatGPT login, and one live fixed-response request.
Only a successful live request establishes runtime verification. Raw CLI output
is suppressed; local tests use fake processes on GitHub-hosted runners.

The [runner guide](ops/private-runner/README.md) defines installation, rotation,
revocation, and recovery. The operator approved the public repository target
provided the triggering user is verified as a maintainer. SSH access to `addons`
confirmed an active repository-scoped runner, Codex 0.158.0, and cached ChatGPT
login. A trusted SSH probe completed a live subscription request. The
[GitHub Actions diagnostic](https://github.com/r59q/hub-rearranger/actions/runs/36456987109)
subsequently passed authorization and the live probe on `addons` at commit
`e7711ffcff53d6ab88e0592c60d2291cf23b9e82`. AW-001 is complete.

The operator explicitly relaxed isolation for this spike on 2026-09-28. The
existing `r59q` service account may be shared with the other runner temporarily.
The probe and native Codex symlink live in `~/.local/lib/hub-agent-runtime`,
outside Actions checkouts, and need no sudo. This makes other jobs under that
account part of the trust boundary; the checksum does not protect against a
compromised shared account. Dedicated-host/account isolation remains the target
before expanding execution beyond the diagnostic. The default-branch and
trusted-actor guards remain in force.

## AW-012 patch execution and shared-runner exception (2026-10-02)

The operator explicitly selected the existing shared `addons` runner/account
for public `r59q/hub-rearranger`. This supersedes the earlier spike-only
restriction for this exact repository. Other public repositories stay blocked;
all default-branch/no-fork/current-maintainer guards remain. Other jobs under
`r59q` remain part of the approved account trust boundary. Workload code still
cannot read authentication, inherit ambient runtime configuration, or use network
access. See the [execution guide](ops/private-runner/EXECUTION.md).

AW-012 extends AW-011 with a hosted read-only `dispatch` verifier and an
operator-installed `patch` job using the profile's `hub-agent-codex` label. Both
gates reconstruct live source/roles/current policy and verify invocation and latest
AW-006 artifact origin/digest. The repository/comment concurrency group stays
unchanged. Prior attempts are reconciled before another workload; verified ready
proposals are reused, while ambiguous/failed/missing runner results block retries.
Only the known AW-011 execution-disabled hosted job proves safe legacy history.

The pinned native Codex CLI uses restricted filesystem permission profiles and
Bubblewrap/seccomp, explicit no-network tool environments, disabled ambient
config/rules/extensions/instructions, and private disposable source/scratch roots.
The auth-bearing harness keeps cached subscription authentication on the host;
workload commands cannot read its auth/process/runtime data. Actual host process
IDs are probed rather than a fresh namespace's harmless `/proc/1`. Readiness is
not inferred from labels, requested flags, or a model-authored response.

Adapter-owned results follow `ops/private-runner/result.schema.v1.json`: exact
identity/base/model policy, validation, safe outcomes, and bounded patch/summary
digests. A private Git index captures actual changes and enforces protected paths,
including intake tooling. Validation runs fixed `make check` in the same offline
sandbox. Raw provider output is bounded/private and never reported. Hosted
`always()` reporting preserves safe observations after ordinary failure or loss;
observations/artifacts never grant publication authority. AW-013 remains the
independent branch/draft-PR write boundary. Both the disabled operator manifest and
repository variable gate must be deliberately enabled after live verification.

The disabled installation on `addons` passed the source-free local preflight on
2026-10-02: real filesystem/network denial and subscription-backed exact
`gpt-6.1-sol`/`high` with Codex 0.159.3, without warnings or fallback. The matching
native code-mode helper is also pinned and installed. Full repository checks
passed. This local proof is separate from the remaining origin-verified AW-006
GitHub diagnostic artifact and controlled Actions patch-job acceptance run.

## AW-009 — Hub bootstrap publication boundary

Agents owns fresh default-branch, commit-pinned canonical bootstrap planning and
review digests. The production image packages AW-008 generator output from the
reviewed root checkout rather than maintaining another template tree. Snapshot
reads reject truncated trees, unsafe path parents/modes and blobs not bound to
their Git IDs. Preview state/diff are context; no durable Hub draft exists.

The opt-in setup page explains every GitHub write and requires explicit consent.
Its bounded same-origin form carries only a base/digest and Identity's session
nonce. Discovery cannot authorize publication. Identity fetches the current public
Agents plan without credentials, compares the review, and performs writes within
AW-019 `WithAuthorization`; it rechecks the App-bound user and current repository
permissions at every write and completion. Account browsing and unrelated views
have no synchronous dependency on bootstrap planning.

The branch is deterministic for user ID/base/digest. Only reviewed canonical paths
may change. Before exposing a ref, Identity verifies the actual full tree, parent,
message and user author/committer. It creates only a dedicated branch and draft PR;
there are no ref updates, merges or source execution. Lost responses and concurrent
attempts reconcile the actual GitHub branch/commit/tree/PR. Changed bases, foreign
or altered branches/PRs, closed PRs and unconfirmed publication fail closed. The
PR body links back to the selected repository/profile setup and contains the
manual dedicated-runner, pinned-runtime, subscription auth, isolation, exact-policy,
offline checks, fresh AW-006 evidence, both gates and artifact-preserving recovery
checklist. Removing Hub leaves GitHub as the saved setup/source of truth.

Validation completed with full `make check` (174 frontend tests plus Go/Python/
Node/workflow/portable-package/contract checks), Agents and Identity race tests,
controlled Chromium desktop/keyboard/dark 390px/320px/native-form checks, and final
isolated Compose builds/health/bundle hashes/outage checks for all five services.
The browser check found and fixed implicit same-host cookie forwarding and a
shared grid minimum-width overflow; read adapters omit implicit credentials,
Identity forwards only the explicit filtered cookie set, and the shared grid
uses a constrained responsive column. No live GitHub sign-in/write or credentialed
runtime was invoked. Real App configuration and manual runner installation remain
operator setup, not hidden Hub state.

## AW-010 — Guided profile authoring

The first editor is scoped to the implemented `codex-thorough` adapter. Its
workflow/diagnostic/operator contract fixes profile ID, runner label and exact
model policy. The UI exposes those settings and role/trigger/authority/validation
with their limitations; it permits explicit display text, enabled-state, context
and review-continuation edits. Images and pipeline workflows stay disabled under
AW-003, and permitted review continuation does not claim AW-016 is implemented.
A broader adapter or model choice requires a coordinated reviewed adapter change.

Agents owns current/default choices and a no-write preview POST. `ProfileDraft`
field shapes derive from the canonical schema, which validates the reconstructed
complete profile. Invalid choices and existing unsupported policy fail safely;
selected mutable fields can change explicitly without overwriting unrelated
profiles or instructions. Repeat plans are stable and no-op catalogs retain their
bytes. Profile-edit digests are distinct from canonical bootstrap intent.

Identity's generated public contract references the Agents draft DTO. It forwards
structured choices without user credentials, obtains a fresh plan, matches the
reviewed base/digest and publishes through `WithAuthorization`, preserving all
AW-009 reauthorization/tree/attribution/ref/PR/retry rules. Authoring PRs link to
the editor, explain changed-policy assignment behavior and retain the manual
runner checklist. The browser supplies no source-file content.

Drafts persist only in repository-scoped browser localStorage or a GitHub draft PR.
Local storage contains authoring choices and a base-revision hint, never session
nonces, credentials or review digests. Changed choices clear review consent;
changed revisions require fresh review. Native forms retain repository selection
and authoring choices on their review page. The tutorial and acknowledgment
precede review; separate explicit consent precedes GitHub publication. No new
service, dependency, environment variable or Compose requirement was introduced.

AW-010 completed on 2026-10-03. Full `make check` passed with 214 frontend tests,
canonical contracts and the portable installation checks; Agents/Identity race
tests, production frontend builds and isolated five-service Compose smoke checks
passed. Controlled Chromium verified browser draft restoration, changed/stale
review recovery, consent/pending/success, failures, keyboard focus, dark mobile
layouts at 390px/320px and no-JavaScript review/publication. Browser testing caught
and fixed named forms dropping repository selection; a native-rendering regression
check now preserves it. Validation used synthetic services/credentials, with no
live GitHub writes, credentialed diagnostic or runner execution.
