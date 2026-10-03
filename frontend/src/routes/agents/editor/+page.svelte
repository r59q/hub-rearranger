<script lang="ts">
	import { browser } from '$app/environment';
	import { resolve } from '$app/paths';
	import { enhance } from '$app/forms';
	import { navigating } from '$app/state';
	import { untrack } from 'svelte';
	import ProfileFields from '$lib/components/agents/ProfileFields.svelte';
	import RunnerSetup from '$lib/components/agents/RunnerSetup.svelte';
	import ProfileReview from '$lib/components/agents/ProfileReview.svelte';
	import { isEditorDraft, type EditorDraft } from '$lib/agents/editor';
	import type { PageProps } from './$types';

	let { data, form }: PageProps = $props();
	let draft = $state<EditorDraft | null>(untrack(() => form?.draft ?? data.editor?.draft ?? null));
	let pending = $state<'review' | 'publish' | null>(null);
	let setupReviewed = $state(false);
	let consent = $state(false);
	let storageNotice = $state('');
	let storageReady = $state(false);
	let storageKey = $derived(`hub-profile-draft:v1:${data.repository?.full_name ?? ''}`);
	let reviewedChoices = $derived(form?.draft ? JSON.stringify(form.draft) : '');
	let changedSinceReview = $derived(!!form?.preview && JSON.stringify(draft) !== reviewedChoices);
	let review = $derived(!changedSinceReview ? form?.preview : null);
	let ready = $derived(review?.state === 'ready' && data.identity.state === 'authenticated');

	// A repository/revision or server form result starts a new authoring context.
	// Only structured choices are saved; session nonces and review grants never are.
	$effect(() => {
		const incoming = form?.draft ?? data.editor?.draft;
		const key = storageKey;
		const revision = data.editor?.baseRevision;
		const serverDraft = form?.draft;
		const completed = form?.result;
		untrack(() => {
			draft = incoming ? structuredClone(incoming) : null;
			setupReviewed = !!serverDraft;
			consent = false;
			storageNotice = '';
			storageReady = false;
			if (browser && draft && !completed) {
				try {
					const saved = localStorage.getItem(key);
					if (saved && !serverDraft) {
						const stored = JSON.parse(saved);
						if (stored.version === 1 && isEditorDraft(stored.draft)) {
							draft = stored.draft;
							storageNotice =
								stored.baseRevision === revision
									? 'Restored your browser draft. Review it before publishing.'
									: 'The GitHub revision changed since this browser draft was saved. Review fresh changes before publishing.';
						}
					}
					storageReady = true;
				} catch {
					storageNotice =
						'Browser draft storage is unavailable. Keep this page open or create a reviewed GitHub draft PR.';
				}
			}
			if (browser && completed) {
				try {
					localStorage.removeItem(key);
				} catch {
					/* The verified PR is already durable on GitHub. */
				}
			}
		});
	});

	$effect(() => {
		if (browser && storageReady && draft && !form?.result) {
			try {
				localStorage.setItem(
					storageKey,
					JSON.stringify({ version: 1, baseRevision: data.editor?.baseRevision, draft })
				);
			} catch {
				storageNotice =
					'Browser draft storage is unavailable. Keep this page open or create a reviewed GitHub draft PR.';
			}
		}
	});

	$effect(() => {
		if (changedSinceReview) {
			consent = false;
		}
	});

	function resetDraft() {
		draft = data.editor?.draft ? structuredClone(data.editor.draft) : null;
		setupReviewed = false;
		consent = false;
		storageNotice = 'Restored the latest loaded GitHub choices. Review again before publishing.';
	}
</script>

<svelte:head><title>Profile editor · Hub Rearranger</title></svelte:head>
<main class="container">
	<header>
		<a
			href={resolve(
				`/agents?${new URLSearchParams({ repository: data.repository?.full_name ?? '' })}`
			)}>Back to agent profiles</a
		>
		<h1>Configure codex-thorough</h1>
		<p>
			Author the repository profile for {data.repository?.full_name ?? 'your workspace'}, review the
			diff, then save it as a GitHub draft PR.
		</p>
	</header>
	{#if navigating.to?.url.pathname === resolve('/agents/editor')}<p role="status" aria-busy="true">
			Loading current profile choices…
		</p>{/if}
	{#if form?.error}<p role="alert" class="notice">{form.error}</p>{/if}
	{#if form?.result}
		<section role="status" aria-labelledby="result-title">
			<h2 id="result-title">Draft PR verified on GitHub</h2>
			<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
			<a href={form.result.pull_request_url}
				>Review draft PR #{form.result.pull_request_number} on GitHub ↗</a
			>
			<p>Branch: <code>{form.result.branch}</code></p>
			<p>
				Review and merge on GitHub, then follow the PR’s manual setup checklist. Hub retains no
				durable draft. Refresh runtime evidence after configuration changes.
			</p>
		</section>
	{:else if data.error}
		<p role="alert" class="notice">{data.error}</p>
	{:else if data.editor?.diagnostics.length}
		<p role="alert">Existing configuration needs review on GitHub before using this editor.</p>
		<ul>
			{#each data.editor.diagnostics as issue, i (i)}<li>
					<code>{issue.path}</code>: {issue.message}
				</li>{/each}
		</ul>
	{:else if data.editor?.draft}
		<p class="muted">
			With JavaScript, choices save only in this browser, scoped to this repository. Without
			JavaScript, submit the native review form to retain choices on this page. GitHub stores the
			configuration once a draft PR is created.
		</p>
		{#if storageNotice}<p role="status">{storageNotice}</p>{/if}
		<form
			method="POST"
			action={`?/review&${new URLSearchParams({ repository: data.repository?.full_name ?? '' })}`}
			use:enhance={() => {
				pending = 'review';
				return async ({ update }) => {
					try {
						await update({ reset: false });
					} finally {
						pending = null;
					}
				};
			}}
		>
			<fieldset disabled={pending !== null}>
				<!-- SSR uses the server draft; hydration restores browser choices afterward. -->
				{#if draft}<ProfileFields bind:draft />{:else}<ProfileFields
						draft={form?.draft ?? data.editor.draft}
					/>{/if}
				<RunnerSetup />
				<label
					><input
						type="checkbox"
						name="setup"
						value="reviewed"
						bind:checked={setupReviewed}
						required
					/> I reviewed the adapter requirements and manual setup steps.</label
				>
				<button type="submit" aria-busy={pending === 'review'}
					>{pending === 'review'
						? 'Generating fresh review…'
						: 'Review configuration changes'}</button
				>
				{#if browser}<button type="button" class="secondary" onclick={resetDraft}
						>Reset to GitHub choices</button
					>{/if}
			</fieldset>
		</form>
		{#if changedSinceReview}<p role="status">
				Your choices changed. Generate a new review before creating a PR.
			</p>{/if}
		{#if review}
			<ProfileReview preview={review} />
			{#if ready}
				<p>Publishing as <strong>{data.identity.user?.login}</strong>.</p>
				<form
					method="POST"
					action={`?/publish&${new URLSearchParams({ repository: data.repository?.full_name ?? '' })}`}
					use:enhance={() => {
						pending = 'publish';
						return async ({ update }) => {
							try {
								await update({ reset: false });
							} finally {
								pending = null;
							}
						};
					}}
				>
					<input type="hidden" name="draft" value={reviewedChoices} />
					<input type="hidden" name="csrf" value={data.identity.csrf ?? ''} />
					<input type="hidden" name="base_revision" value={review.base_revision} />
					<input type="hidden" name="digest" value={review.digest} />
					<label
						><input
							type="checkbox"
							name="confirm"
							value="reviewed"
							bind:checked={consent}
							required
							disabled={pending !== null}
						/> I reviewed this diff and approve the listed GitHub writes.</label
					>
					<button
						type="submit"
						disabled={pending !== null || (browser && !consent)}
						aria-busy={pending === 'publish'}
						>{pending === 'publish'
							? 'Creating and verifying draft PR…'
							: 'Create configuration draft PR'}</button
					>
				</form>
			{:else if data.identity.state !== 'authenticated'}
				<p class="notice">
					{data.identity.state === 'disabled'
						? 'The Hub operator must configure GitHub sign-in before publication.'
						: data.identity.state === 'unavailable'
							? 'Identity is unavailable. Retry when your GitHub connection can be checked.'
							: 'Connect or reconnect your GitHub account before publication.'}
					<a href={resolve('/account')}>GitHub connection</a>
				</p>
			{/if}
		{/if}
	{/if}
</main>

<style>
	main {
		max-width: 58rem;
		padding-block: 2rem;
	}
	header {
		margin-bottom: 2rem;
	}
	fieldset {
		min-width: 0;
	}
	.notice {
		padding: 1rem;
		border: 1px solid var(--pico-muted-border-color);
		border-radius: var(--pico-border-radius);
	}
	.muted {
		color: var(--pico-muted-color);
	}
	code,
	li {
		overflow-wrap: anywhere;
	}
</style>
