<script lang="ts">
	import type { SubmitFunction } from '@sveltejs/kit';
	import { createOptimisticSubmit } from '$lib/forms/optimistic-submit';
	import { filterRepositories } from '$lib/repositories/filter-repositories';
	import type { Repository } from '$lib/repositories/types';
	import AddRepositoryTile from './AddRepositoryTile.svelte';

	let {
		repositories,
		onOptimisticAdd,
		onOptimisticSettled
	}: {
		repositories: Repository[];
		onOptimisticAdd: (repository: Repository) => void;
		onOptimisticSettled: (repositoryId: number) => void;
	} = $props();
	let query = $state('');
	let addingId = $state<number | null>(null);
	let filteredRepositories = $derived(filterRepositories(repositories, query));

	function enhanceAdd(repository: Repository): SubmitFunction {
		return createOptimisticSubmit({
			value: repository,
			onMutate: (pendingRepository) => {
				addingId = pendingRepository.id;
				onOptimisticAdd(pendingRepository);
			},
			onSettled: (settledRepository) => {
				onOptimisticSettled(settledRepository.id);
				addingId = null;
			}
		});
	}
</script>

<section class="picker" aria-labelledby="available-title">
	<header>
		<div>
			<p class="eyebrow">GitHub account</p>
			<h2 id="available-title">Add repositories</h2>
			<p>Choose repositories to bring into this workspace. This does not change GitHub.</p>
		</div>
		{#if repositories.length > 0}
			<label class="search">
				<span class="sr-only">Search repositories by name</span>
				<svg viewBox="0 0 24 24" aria-hidden="true">
					<circle cx="11" cy="11" r="6.5" /><path d="m16 16 4 4" />
				</svg>
				<input
					bind:value={query}
					type="search"
					placeholder="Search repositories"
					autocomplete="off"
				/>
			</label>
		{/if}
	</header>

	{#if repositories.length === 0}
		<div class="picker-state">
			<span aria-hidden="true">✓</span>
			<div>
				<strong>Everything is added</strong>
				<p>There are no more repositories available to add.</p>
			</div>
		</div>
	{:else if filteredRepositories.length === 0}
		<div class="picker-state" aria-live="polite">
			<span aria-hidden="true">?</span>
			<div>
				<strong>No repositories found</strong>
				<p>Try another repository name.</p>
			</div>
		</div>
	{:else}
		<p class="result-count" aria-live="polite">
			{filteredRepositories.length}
			{filteredRepositories.length === 1 ? 'repository' : 'repositories'} available
		</p>
		<div class="repository-options">
			{#each filteredRepositories as repository (repository.id)}
				<AddRepositoryTile
					{repository}
					submit={enhanceAdd(repository)}
					busy={addingId === repository.id}
					disabled={addingId !== null}
				/>
			{/each}
		</div>
	{/if}
</section>

<style>
	.picker {
		margin-top: clamp(3rem, 7vw, 5rem);
		padding-top: clamp(2rem, 5vw, 3.5rem);
		border-top: 1px solid var(--pico-muted-border-color);
	}

	header {
		display: flex;
		align-items: flex-end;
		justify-content: space-between;
		gap: 2rem;
		margin-bottom: 1.5rem;
	}

	.eyebrow {
		margin: 0 0 0.35rem;
		color: var(--pico-primary);
		font-size: 0.7rem;
		font-weight: 750;
		letter-spacing: 0.13em;
		text-transform: uppercase;
	}

	h2 {
		margin: 0;
		font-size: 1.75rem;
		letter-spacing: -0.03em;
	}

	header p:last-child {
		margin: 0.45rem 0 0;
		color: var(--pico-muted-color);
		font-size: 0.9rem;
	}

	.search {
		position: relative;
		width: min(20rem, 100%);
		margin: 0;
	}

	.search svg {
		position: absolute;
		top: 50%;
		left: 0.9rem;
		width: 1rem;
		transform: translateY(-50%);
		fill: none;
		stroke: var(--pico-muted-color);
		stroke-linecap: round;
		stroke-width: 1.8;
		pointer-events: none;
	}

	.search input {
		margin: 0;
		padding-left: 2.5rem;
		background: var(--pico-card-background-color);
	}

	.result-count {
		margin: 0 0 0.75rem;
		color: var(--pico-muted-color);
		font-size: 0.76rem;
	}

	.repository-options {
		display: grid;
		gap: 0.85rem;
		grid-template-columns: repeat(2, minmax(0, 1fr));
	}

	.picker-state {
		display: flex;
		align-items: center;
		gap: 0.9rem;
		padding: 1.25rem;
		border: 1px dashed var(--pico-muted-border-color);
		border-radius: var(--pico-border-radius);
		color: var(--pico-muted-color);
	}

	.picker-state > span {
		display: grid;
		width: 2rem;
		height: 2rem;
		flex: 0 0 auto;
		place-items: center;
		border-radius: 50%;
		background: color-mix(in srgb, var(--pico-primary) 10%, transparent);
		color: var(--pico-primary);
		font-weight: 800;
	}

	.picker-state strong {
		color: var(--pico-color);
	}

	.picker-state p {
		margin: 0.1rem 0 0;
		font-size: 0.82rem;
	}

	.sr-only {
		position: absolute;
		width: 1px;
		height: 1px;
		padding: 0;
		overflow: hidden;
		clip: rect(0, 0, 0, 0);
		white-space: nowrap;
		border: 0;
	}

	@media (max-width: 760px) {
		header {
			align-items: stretch;
			flex-direction: column;
			gap: 1.25rem;
		}

		.search {
			width: 100%;
		}

		.repository-options {
			grid-template-columns: 1fr;
		}
	}
</style>
