<script lang="ts">
	import { resolve } from '$app/paths';
	import { invalidateAll } from '$app/navigation';
	import { navigating } from '$app/state';
	import AgentWorkspaceView from '$lib/components/agents/AgentWorkspaceView.svelte';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	let refreshing = $state(false);
	let refreshError = $state<string | null>(null);
	let busy = $derived(refreshing || navigating.to?.url.pathname === resolve('/agents'));

	async function refresh(event: SubmitEvent) {
		event.preventDefault();
		refreshing = true;
		refreshError = null;

		try {
			await invalidateAll();
		} catch {
			refreshError = 'Profiles could not be refreshed. Try again.';
		} finally {
			refreshing = false;
		}
	}
</script>

<svelte:head>
	<title>Agents · Hub Rearranger</title>
	<meta
		name="description"
		content="Review repository agent profiles, authority, required runners, and runtime readiness."
	/>
</svelte:head>

<main class="container agents-page">
	<header class="page-heading">
		<p class="eyebrow">Workdesk</p>
		<h1>Agents</h1>
		<p>Understand the policy and runtime requirements for each repository’s agent profiles.</p>
	</header>
	{#if data.serviceError}
		<section class="agent-notice error" role="alert">
			<h2>Repositories are unavailable</h2>
			<p>{data.serviceError}</p>
			<a href={resolve('/agents')} data-sveltekit-reload>Try again</a>
		</section>
	{:else if data.repositories.length === 0}
		<section class="agent-notice">
			<h2>Choose a repository first</h2>
			<p>Add a repository to your workspace to view its agent profiles and readiness.</p>
			<a href={resolve('/')}>Choose repositories</a>
		</section>
	{:else}
		<div class="repository-controls">
			<form method="GET" action={resolve('/agents')} class="repository-form">
				<label for="agent-repository">Repository</label>
				<div class="selector">
					<select id="agent-repository" name="repository" disabled={busy}>
						{#each data.repositories as repository (repository.id)}<option
								value={repository.full_name}
								selected={repository.id === data.repository?.id}>{repository.full_name}</option
							>{/each}
					</select>
					<button type="submit" class="secondary" disabled={busy}>View profiles</button>
				</div>
			</form>
			{#if data.repository}
				<a
					href={resolve(
						`/agents/bootstrap?${new URLSearchParams({ repository: data.repository.full_name })}`
					)}>Review bootstrap setup</a
				>
				<a
					href={resolve(
						`/agents/editor?${new URLSearchParams({ repository: data.repository.full_name })}`
					)}>Edit profile and setup</a
				>
				<form method="GET" action={resolve('/agents')} onsubmit={refresh} class="refresh-form">
					<input type="hidden" name="repository" value={data.repository.full_name} />
					<button type="submit" class="outline" aria-busy={refreshing} disabled={busy}
						>{refreshing ? 'Refreshing…' : 'Refresh profiles'}</button
					>
				</form>
			{/if}
		</div>
		{#if data.selectionError}<p class="agent-notice error" role="alert">
				{data.selectionError}
			</p>{/if}
		{#if refreshError}<p class="agent-notice error" role="alert">{refreshError}</p>{/if}
		{#if data.workspace}
			{#await data.workspace}
				<section class="agent-notice loading" role="status" aria-busy="true">
					<h2>Loading profiles and readiness…</h2>
					<p>
						Reading {data.repository?.full_name} from GitHub. Readiness may take a little longer.
					</p>
				</section>
			{:then workspace}
				{#if workspace}
					{#key workspace.repository + workspace.loadedAt}<AgentWorkspaceView
							{workspace}
							{busy}
							refreshFailed={Boolean(refreshError)}
						/>{/key}
				{/if}
			{/await}
		{/if}
	{/if}
</main>

<style>
	main {
		width: 100%;
		padding-block: clamp(2.25rem, 5vw, 4.5rem) 5rem;
	}
	.page-heading {
		margin-bottom: 2rem;
	}
	.eyebrow {
		margin: 0 0 0.5rem;
		color: var(--pico-primary);
		font-size: 0.72rem;
		font-weight: 750;
		letter-spacing: 0.13em;
		text-transform: uppercase;
	}
	h1 {
		margin: 0;
		font-size: clamp(2.35rem, 5vw, 3.75rem);
		letter-spacing: -0.045em;
		line-height: 1.05;
	}
	.page-heading > p:last-child {
		margin: 0.75rem 0 0;
		color: var(--pico-muted-color);
	}
	.repository-controls {
		display: flex;
		align-items: flex-end;
		gap: 1rem;
		margin-bottom: 2rem;
		padding-bottom: 2rem;
		border-bottom: 1px solid var(--pico-muted-border-color);
	}
	form {
		margin: 0;
	}
	.repository-form {
		flex: 1;
		min-width: 0;
	}
	label {
		font-size: 0.85rem;
	}
	.selector {
		display: grid;
		grid-template-columns: minmax(0, 1fr) auto;
		gap: 0.75rem;
	}
	select,
	button {
		margin: 0;
		font-size: 0.85rem;
		min-height: 2.75rem;
	}
	.refresh-form {
		flex: 0 0 auto;
	}
	.loading h2 {
		font-size: 1.1rem;
	}
	:global(.agents-page .agent-notice) {
		margin: 0 0 1.5rem;
		padding: 1.25rem;
		border: 1px solid var(--pico-muted-border-color);
		border-radius: var(--pico-border-radius);
		background: var(--pico-card-background-color);
		overflow-wrap: anywhere;
		font-size: 0.9rem;
	}
	:global(.agents-page .agent-notice.error) {
		border-left: 3px solid var(--pico-del-color);
	}
	:global(.agents-page .agent-notice h2),
	:global(.agents-page .agent-notice h3) {
		margin: 0 0 0.5rem;
		font-size: 1.15rem;
	}
	:global(.agents-page .agent-notice p:last-child),
	:global(.agents-page .agent-notice ul:last-child) {
		margin-bottom: 0;
	}
	:global(.agents-page .agent-notice code) {
		white-space: normal;
		overflow-wrap: anywhere;
	}
	@media (max-width: 700px) {
		.repository-controls {
			align-items: stretch;
			flex-direction: column;
		}
		.selector {
			grid-template-columns: 1fr;
		}
		.refresh-form button {
			width: 100%;
		}
	}
</style>
