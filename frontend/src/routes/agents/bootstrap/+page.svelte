<script lang="ts">
	import { resolve } from '$app/paths';
	import { enhance } from '$app/forms';
	import { navigating } from '$app/state';
	import type { PageProps } from './$types';

	let { data, form }: PageProps = $props();
	let pending = $state(false);
	let confirmed = $state(false);
	let ready = $derived(data.preview?.state === 'ready' && data.identity.state === 'authenticated');
	let changed = $derived(
		data.preview?.files.filter((file) => file.status === 'create' || file.status === 'update') ?? []
	);
</script>

<svelte:head><title>Bootstrap review · Hub Rearranger</title></svelte:head>
<main class="container">
	<header>
		<a
			href={resolve(
				`/agents?${new URLSearchParams({ repository: data.repository?.full_name ?? '' })}`
			)}>Back to agent profiles</a
		>
		<h1>Set up codex-thorough</h1>
		<p>
			Review the GitHub-owned configuration for {data.repository?.full_name ?? 'your repository'}.
		</p>
	</header>
	{#if navigating.to?.url.pathname === resolve('/agents/bootstrap')}
		<p role="status" aria-busy="true">Loading a fresh bootstrap review…</p>
	{/if}
	{#if form?.error}<p role="alert" class="notice">
			{form.error} <a href={resolve('/account')}>Check GitHub connection</a>
		</p>{/if}
	{#if form?.result}
		<section aria-labelledby="success-title" role="status">
			<h2 id="success-title">Draft PR verified on GitHub</h2>
			<!-- The server adapter verifies the exact canonical GitHub PR URL. -->
			<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
			<a href={form.result.pull_request_url}
				>Review draft PR #{form.result.pull_request_number} on GitHub ↗</a
			>
			<p>Branch: <code>{form.result.branch}</code></p>
			<p>
				Review and merge the configuration on GitHub, then follow the PR’s manual runner checklist.
				The PR and files are the saved setup; Hub retains no bootstrap draft.
			</p>
		</section>
	{:else if data.error}
		<p role="alert" class="notice">{data.error}</p>
	{:else if data.preview}
		<section aria-labelledby="review-title">
			<h2 id="review-title">Proposed repository changes</h2>
			<p>
				Default branch: <code>{data.preview.default_branch}</code> · reviewed commit:
				<code>{data.preview.base_revision}</code>
			</p>
			{#if data.preview.state === 'unchanged'}
				<p role="status">
					The bootstrap files already match. No PR is needed. Runtime verification remains a
					separate step.
				</p>
			{:else if data.preview.state === 'conflict'}
				<p role="alert">Existing configuration needs a decision before a PR can be created.</p>
				<ul>
					{#each data.preview.diagnostics as diagnostic, index (index)}<li>
							<code>{diagnostic.path}</code>: {diagnostic.message}
						</li>{/each}
				</ul>
			{:else}
				<p>
					{changed.length} files will be created or updated. Existing repository instructions and compatible
					profiles are preserved in the proposed diff.
				</p>
			{/if}
			<details>
				<summary>File changes ({changed.length})</summary>
				<ul>
					{#each data.preview.files as file (file.path)}<li>
							<code>{file.path}</code> — {file.status}
						</li>{/each}
				</ul>
			</details>
			<details>
				<summary>Review the full Git diff</summary>
				<!-- svelte-ignore a11y_no_noninteractive_tabindex (Keyboard users need to focus and scroll this bounded diff.) -->
				<pre role="region" tabindex="0" aria-label="Proposed Git diff">{data.preview.diff ||
						'No changes.'}</pre>
			</details>
			<p class="muted">
				The review is pinned to this commit and these exact proposed bytes. Reload to refresh it; a
				changed default branch requires a new review.
			</p>
		</section>
		<section aria-labelledby="writes-title">
			<h2 id="writes-title">GitHub writes you are approving</h2>
			<ol>
				<li>Create Git file/tree objects containing only the reviewed bootstrap changes.</li>
				<li>
					Create one commit attributed to your signed-in GitHub identity, based on the reviewed
					default-branch commit.
				</li>
				<li>
					Create a dedicated <code>hub-bootstrap/codex-thorough-…</code> branch pointing to that commit.
				</li>
				<li>
					Create a draft PR into <code>{data.preview.default_branch}</code>, with a link to this
					setup and the manual runner checklist.
				</li>
			</ol>
			<p>
				Hub rechecks your repository write access and the App’s Contents, Pull requests and
				Workflows permissions during publication. A retry verifies any existing result before
				continuing.
			</p>
			<p>
				You review and merge the PR on GitHub. Runner installation and execution enablement remain
				manual; creating this PR does not start an agent.
			</p>
		</section>
		<section aria-labelledby="runner-title">
			<h2 id="runner-title">Manual setup after review</h2>
			<p>
				Provision a dedicated restricted host/account for an approved private repository with the <code
					>hub-agent-codex</code
				> label. Public repositories require separate operator approval; the upstream exception does not
				transfer.
			</p>
			<p>
				Install the pinned runtime and subscription-backed Codex outside checkouts. Verify
				isolation, disabled workload networking and exact <code>gpt-6.1-sol/high</code> policy.
				Provide the root offline <code>make check</code> command, merge the reviewed files, then run the
				no-write diagnostic.
			</p>
			<p>
				Verify fresh evidence for the current revision before enabling both the private operator
				manifest and repository execution gate. The PR and installed documentation contain the full
				checklist, credential rotation and recovery steps.
			</p>
		</section>
		{#if data.identity.state === 'authenticated'}
			<p>Publishing as <strong>{data.identity.user?.login}</strong>.</p>
		{:else}
			<p class="notice">
				{data.identity.state === 'disabled'
					? 'The Hub operator must configure GitHub sign-in before publication.'
					: data.identity.state === 'unavailable'
						? 'GitHub connection could not be checked. Retry when Identity is available.'
						: 'Connect or reconnect your GitHub account before creating a PR.'}
				<a href={resolve('/account')}>GitHub connection</a>
			</p>
		{/if}
		{#if ready}
			<form
				method="POST"
				use:enhance={() => {
					pending = true;
					return async ({ update }) => {
						try {
							await update();
						} finally {
							pending = false;
							confirmed = false;
						}
					};
				}}
				aria-busy={pending}
			>
				<input type="hidden" name="csrf" value={data.identity.csrf ?? ''} />
				<input type="hidden" name="base_revision" value={data.preview.base_revision} />
				<input type="hidden" name="digest" value={data.preview.digest} />
				<label
					><input
						type="checkbox"
						name="confirm"
						value="reviewed"
						required
						bind:checked={confirmed}
						disabled={pending}
					/> I reviewed the diff and approve the GitHub writes listed above.</label
				>
				<button disabled={pending} aria-busy={pending}
					>{pending ? 'Creating and verifying draft PR…' : 'Create bootstrap draft PR'}</button
				>
			</form>
		{/if}
	{/if}
	<p>
		<a
			href={resolve(
				`/agents/bootstrap?${new URLSearchParams({ repository: data.repository?.full_name ?? '' })}`
			)}
			data-sveltekit-reload>Reload current bootstrap review</a
		>
	</p>
</main>

<style>
	main {
		width: 100%;
		min-width: 0;
		max-width: 60rem;
		padding-block: 2.5rem 5rem;
	}
	header {
		margin-bottom: 2rem;
	}
	h1 {
		margin-top: 1rem;
		font-size: clamp(2rem, 5vw, 3rem);
	}
	h2 {
		font-size: 1.3rem;
	}
	section,
	.notice {
		border: 1px solid var(--pico-muted-border-color);
		background: var(--pico-card-background-color);
		border-radius: var(--pico-border-radius);
		padding: clamp(1rem, 3vw, 1.5rem);
		margin-bottom: 1.5rem;
	}
	p,
	li,
	code {
		overflow-wrap: anywhere;
	}
	p,
	li {
		font-size: 0.9rem;
	}
	li {
		margin-bottom: 0.6rem;
	}
	code {
		white-space: normal;
	}
	pre {
		max-height: 35rem;
		overflow: auto;
		font-size: 0.75rem;
	}
	summary,
	button {
		min-height: 2.75rem;
	}
	summary {
		padding-block: 0.75rem;
	}
	.muted {
		color: var(--pico-muted-color);
		font-size: 0.8rem;
	}
	label {
		margin-bottom: 1.25rem;
	}
	@media (max-width: 600px) {
		button {
			width: 100%;
			white-space: normal;
		}
	}
</style>
