<script lang="ts">
	import type { AgentWorkspace } from '$lib/agents/types';
	import { formatTimestamp, workspaceIsStale } from '$lib/agents/presentation';
	import AgentProfileCard from './AgentProfileCard.svelte';

	let {
		workspace,
		busy = false,
		refreshFailed = false
	}: { workspace: AgentWorkspace; busy?: boolean; refreshFailed?: boolean } = $props();

	let clock = $state<number | null>(null);
	let stale = $derived(
		refreshFailed || workspaceIsStale(workspace, clock ?? Date.parse(workspace.loadedAt))
	);

	$effect(() => {
		const updateClock = () => {
			clock = Date.now();
		};
		updateClock();

		const interval = window.setInterval(updateClock, 30_000);
		window.addEventListener('focus', updateClock);
		document.addEventListener('visibilitychange', updateClock);

		return () => {
			window.clearInterval(interval);
			window.removeEventListener('focus', updateClock);
			document.removeEventListener('visibilitychange', updateClock);
		};
	});
</script>

<section aria-labelledby="repository-profiles-title" aria-busy={busy}>
	<header class="workspace-heading">
		<div>
			<h2 id="repository-profiles-title">{workspace.repository}</h2>
			{#if workspace.revision}
				<p class="snapshot">
					{workspace.branch} ·
					<!-- These links point to fixed GitHub objects, never application routes. -->
					<!-- eslint-disable svelte/no-navigation-without-resolve -->
					<a
						href={workspace.commitUrl}
						target="_blank"
						rel="noreferrer"
						title={'Commit ' + workspace.revision}><code>{workspace.revision.slice(0, 12)}</code></a
					>
				</p>
			{/if}
		</div>
		{#if workspace.catalogUrl}
			<!-- eslint-disable svelte/no-navigation-without-resolve -->
			<a href={workspace.catalogUrl} target="_blank" rel="noreferrer">Profile catalog on GitHub ↗</a
			>
		{/if}
	</header>
	{#if busy}
		<p class="agent-notice" role="status">
			Refreshing profiles and readiness… Previous results are shown below.
		</p>
	{:else if stale}
		<p class="agent-notice" role="status">
			Results need refreshing. {workspace.inconsistent
				? 'Configuration and readiness came from different repository snapshots.'
				: 'This view may no longer reflect the current repository or diagnostic evidence.'} Refresh profiles
			before relying on runtime status.
		</p>
	{/if}

	{#if workspace.error}
		<div class="agent-notice error" role="alert">
			<h3>Profiles are unavailable</h3>
			<p>{workspace.error}</p>
		</div>
	{:else if workspace.state !== 'ready'}
		<div class="agent-notice" role="status">
			<h3>
				{workspace.state === 'catalog_missing'
					? 'No agent profiles configured'
					: workspace.state === 'catalog_unsupported'
						? 'Profile catalog version unsupported'
						: 'Profile catalog needs attention'}
			</h3>
			<p>
				{workspace.state === 'catalog_missing'
					? 'Add .github/agent-profiles.yml to the default branch to define the repository’s agent profiles.'
					: 'Fix the catalog on GitHub, then refresh profiles to read its configuration.'}
			</p>
			{#if workspace.diagnostics.length > 0}
				<ul>
					{#each workspace.diagnostics as diagnostic, index (index)}<li>
							<code>{diagnostic.path}</code> — {diagnostic.message}
						</li>{/each}
				</ul>
			{/if}
		</div>
	{:else}
		{#if workspace.readinessError}
			<div class="agent-notice error" role="alert">
				<h3>Readiness is unavailable</h3>
				<p>{workspace.readinessError}</p>
				<p>Profile policy is still shown at the displayed revision.</p>
			</div>
		{/if}
		<p class="result-count" role="status">
			{workspace.profiles.length}
			{workspace.profiles.length === 1 ? 'profile' : 'profiles'} · Loaded
			<time datetime={workspace.loadedAt}>{formatTimestamp(workspace.loadedAt)}</time>
		</p>
		<div class="profiles">
			{#each workspace.profiles as profile (profile.id)}<AgentProfileCard
					{profile}
					stale={stale || busy}
				/>{/each}
		</div>
		<p class="diagnostic-link">
			<!-- eslint-disable svelte/no-navigation-without-resolve -->
			<a href={workspace.diagnosticUrl} target="_blank" rel="noreferrer"
				>Profile diagnostic workflow on GitHub ↗</a
			>
			<span>Runtime verification applies to the displayed revision and expires after 24 hours.</span
			>
		</p>
	{/if}
</section>

<style>
	.workspace-heading {
		display: flex;
		align-items: flex-start;
		justify-content: space-between;
		gap: 1.25rem;
		margin: 0 0 1.5rem;
	}
	.workspace-heading > div {
		min-width: 0;
	}
	h2 {
		margin: 0;
		font-size: 1.35rem;
		overflow-wrap: anywhere;
	}
	.workspace-heading a {
		font-size: 0.8rem;
	}
	.snapshot {
		margin: 0.45rem 0 0;
		color: var(--pico-muted-color);
		font-size: 0.8rem;
		overflow-wrap: anywhere;
	}
	.snapshot code {
		font-size: 0.75rem;
	}
	.profiles {
		display: grid;
		gap: 1.25rem;
	}
	.result-count {
		color: var(--pico-muted-color);
		font-size: 0.8rem;
		margin: 0 0 1rem;
	}
	.diagnostic-link {
		display: flex;
		flex-wrap: wrap;
		justify-content: space-between;
		gap: 0.75rem 1.25rem;
		margin: 1.5rem 0 0;
		font-size: 0.8rem;
	}
	.diagnostic-link span {
		color: var(--pico-muted-color);
	}
	@media (max-width: 700px) {
		.workspace-heading {
			flex-direction: column;
		}
	}
</style>
