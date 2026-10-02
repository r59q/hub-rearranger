<script lang="ts">
	import { resolve } from '$app/paths';
	import { profilesQuery } from '$lib/agents/presentation';
	import type { Repository } from '$lib/repositories/types';

	let { repository, pending = false }: { repository: Repository; pending?: boolean } = $props();
</script>

<article>
	<header>
		<div class="repo-icon" aria-hidden="true">
			<svg viewBox="0 0 24 24">
				<path
					d="M5 3.75h11.25A2.75 2.75 0 0 1 19 6.5v12.75H7.75A2.75 2.75 0 0 1 5 16.5V3.75Zm0 12.75a2.75 2.75 0 0 1 2.75-2.75H19M8.5 7.5h7"
				/>
			</svg>
		</div>
		{#if pending}
			<span class="visibility pending" aria-live="polite" aria-busy="true">Adding</span>
		{:else}
			<span class="visibility">{repository.private ? 'Private' : 'Public'}</span>
		{/if}
	</header>
	<div class="repo-copy">
		<small>{repository.owner}</small>
		<h2>{repository.name}</h2>
		<p>{repository.description || 'No description provided on GitHub.'}</p>
	</div>
	{#if !pending}
		<!-- The base-aware resolved Agents route is followed by an encoded query string. -->
		<!-- eslint-disable svelte/no-navigation-without-resolve -->
		<a class="agents-link" href={resolve('/agents') + profilesQuery(repository.full_name)}
			>Agent profiles <span aria-hidden="true">→</span></a
		>
		<!-- eslint-enable svelte/no-navigation-without-resolve -->
	{/if}
	<footer>
		<span class="branch" title="Default branch">
			<svg viewBox="0 0 24 24" aria-hidden="true"
				><path d="M6 3v12a3 3 0 0 0 3 3h12M16 14l4 4-4 4M18 9V3" /></svg
			>
			{repository.default_branch || 'No default branch'}
		</span>
		<!-- GitHub URLs are API data, not application routes. -->
		<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
		<a href={repository.html_url} target="_blank" rel="noreferrer">
			Open on GitHub
			<svg viewBox="0 0 24 24" aria-hidden="true"
				><path d="M14 5h5v5M19 5l-8 8M19 14v5H5V5h5" /></svg
			>
		</a>
	</footer>
</article>

<style>
	article {
		display: flex;
		min-height: 17rem;
		flex-direction: column;
		margin: 0;
		padding: 1.35rem;
		border: 1px solid var(--pico-muted-border-color);
		box-shadow: 0 0.7rem 2rem color-mix(in srgb, var(--pico-color) 4%, transparent);
		transition:
			border-color 160ms ease,
			transform 160ms ease;
	}

	article:hover {
		border-color: color-mix(in srgb, var(--pico-primary) 45%, var(--pico-muted-border-color));
		transform: translateY(-2px);
	}

	header,
	footer {
		display: flex;
		align-items: center;
		justify-content: space-between;
	}

	.repo-icon {
		display: grid;
		width: 2.6rem;
		height: 2.6rem;
		place-items: center;
		border-radius: 0.65rem;
		background: color-mix(in srgb, var(--pico-primary) 10%, transparent);
		color: var(--pico-primary);
	}

	.visibility {
		padding: 0.2rem 0.55rem;
		border: 1px solid var(--pico-muted-border-color);
		border-radius: 2rem;
		color: var(--pico-muted-color);
		font-size: 0.7rem;
		font-weight: 650;
	}

	.visibility.pending {
		border-color: color-mix(in srgb, var(--pico-primary) 35%, var(--pico-muted-border-color));
		color: var(--pico-primary);
	}

	.repo-copy {
		flex: 1;
		padding-block: 1.5rem;
	}

	.repo-copy small,
	.repo-copy p,
	.branch {
		color: var(--pico-muted-color);
	}

	h2 {
		margin: 0.15rem 0 0.75rem;
		font-size: 1.35rem;
		letter-spacing: -0.02em;
	}

	.repo-copy p {
		display: -webkit-box;
		overflow: hidden;
		margin: 0;
		-webkit-box-orient: vertical;
		-webkit-line-clamp: 2;
		line-clamp: 2;
		font-size: 0.88rem;
	}

	.agents-link {
		display: inline-flex;
		align-items: center;
		gap: 0.35rem;
		min-height: 2.75rem;
		margin-bottom: 1rem;
		font-size: 0.85rem;
		font-weight: 650;
		width: fit-content;
	}
	footer {
		gap: 1rem;
		padding-top: 1rem;
		border-top: 1px solid var(--pico-muted-border-color);
		font-size: 0.76rem;
	}

	.branch,
	footer a {
		display: inline-flex;
		min-width: 0;
		align-items: center;
		gap: 0.35rem;
	}

	.branch {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	footer a {
		flex: 0 0 auto;
		font-weight: 650;
		text-decoration: none;
	}

	svg {
		width: 1.1rem;
		height: 1.1rem;
		fill: none;
		stroke: currentColor;
		stroke-linecap: round;
		stroke-linejoin: round;
		stroke-width: 1.8;
	}

	@media (max-width: 700px) {
		article {
			min-height: 15rem;
		}
	}
</style>
