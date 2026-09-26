<script lang="ts">
	import { resolve } from '$app/paths';
	import AppFooter from '$lib/components/AppFooter.svelte';
	import AppHeader from '$lib/components/AppHeader.svelte';
	import RepositoryCard from '$lib/components/RepositoryCard.svelte';
	import RepositoryPicker from '$lib/components/RepositoryPicker.svelte';
	import { mergeOptimisticRepositories } from '$lib/repositories/optimistic-repositories';
	import type { Repository } from '$lib/repositories/types';
	import type { PageProps } from './$types';

	let { data, form }: PageProps = $props();
	let optimisticRepositories = $state<Repository[]>([]);
	let visibleRepositories = $derived(
		mergeOptimisticRepositories(data.repositories, optimisticRepositories)
	);
	let unselectedRepositories = $derived(
		data.availableRepositories.filter(
			(repository) =>
				!repository.selected && !optimisticRepositories.some(({ id }) => id === repository.id)
		)
	);

	function addOptimistically(repository: Repository) {
		if (optimisticRepositories.some(({ id }) => id === repository.id)) return;
		optimisticRepositories = [...optimisticRepositories, repository];
	}

	function settleOptimisticRepository(repositoryId: number) {
		optimisticRepositories = optimisticRepositories.filter(({ id }) => id !== repositoryId);
	}
</script>

<svelte:head>
	<title>Repositories · Hub Rearranger</title>
	<meta name="description" content="Choose the GitHub repositories in your focused workspace." />
</svelte:head>

<div class="site-shell">
	<AppHeader />

	<main class="container">
		<section class="page-heading" aria-labelledby="page-title">
			<p class="eyebrow">Workspace</p>
			<h1 id="page-title">Repositories</h1>
			<p>Keep the repositories that matter close at hand.</p>
		</section>

		{#if form?.message}
			<div class:success={form.success} class="notice" role={form.success ? 'status' : 'alert'}>
				<span>{form.message}</span>
				<button type="button" aria-label="Dismiss message" onclick={() => (form = null)}>×</button>
			</div>
		{/if}

		{#if data.serviceError}
			<section class="state-card error-state" aria-labelledby="service-error-title">
				<span class="state-icon" aria-hidden="true">!</span>
				<div>
					<h2 id="service-error-title">Repositories are unavailable</h2>
					<p>{data.serviceError}</p>
					<a href={resolve('/')}>Try again</a>
				</div>
			</section>
		{:else}
			<section aria-labelledby="workspace-title">
				<div class="repository-summary">
					<div>
						<h2 id="workspace-title">Your workspace</h2>
						<p>
							<strong>{visibleRepositories.length}</strong>
							{visibleRepositories.length === 1 ? 'repository' : 'repositories'} selected
						</p>
					</div>
					<small>Repository details stay in sync with GitHub.</small>
				</div>

				{#if visibleRepositories.length === 0}
					<div class="empty-state">
						<div class="empty-illustration" aria-hidden="true">
							<svg viewBox="0 0 96 96">
								<path
									d="M20 29.5A9.5 9.5 0 0 1 29.5 20h37A9.5 9.5 0 0 1 76 29.5V76H29.5a9.5 9.5 0 0 1-9.5-9.5v-37Z"
								/>
								<path d="M20 65.5a9.5 9.5 0 0 1 9.5-9.5H76M32 33h27M32 44h18" />
							</svg>
						</div>
						<div>
							<h3>Build your workspace</h3>
							<p>Add a repository from your GitHub account below.</p>
						</div>
					</div>
				{:else}
					<div class="repository-grid">
						{#each visibleRepositories as repository (repository.id)}
							<RepositoryCard
								{repository}
								pending={optimisticRepositories.some(({ id }) => id === repository.id)}
							/>
						{/each}
					</div>
				{/if}
			</section>

			<RepositoryPicker
				repositories={unselectedRepositories}
				onOptimisticAdd={addOptimistically}
				onOptimisticSettled={settleOptimisticRepository}
			/>
		{/if}
	</main>

	<AppFooter />
</div>

<style>
	.site-shell {
		display: grid;
		min-height: 100vh;
		grid-template-rows: auto 1fr auto;
		background:
			radial-gradient(
				circle at 82% 0%,
				color-mix(in srgb, var(--pico-primary) 7%, transparent),
				transparent 24rem
			),
			var(--pico-background-color);
	}

	main {
		width: 100%;
		padding-block: clamp(2.25rem, 5vw, 4.5rem) 5rem;
	}

	.page-heading {
		margin-bottom: 2.75rem;
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

	.notice {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 1rem;
		margin-bottom: 1.5rem;
		padding: 0.8rem 1rem;
		border: 1px solid color-mix(in srgb, var(--pico-del-color) 45%, transparent);
		border-radius: var(--pico-border-radius);
		background: color-mix(in srgb, var(--pico-del-color) 9%, transparent);
	}

	.notice.success {
		border-color: color-mix(in srgb, var(--pico-primary) 40%, transparent);
		background: color-mix(in srgb, var(--pico-primary) 8%, transparent);
	}

	.notice button {
		width: auto;
		margin: 0;
		padding: 0.15rem 0.45rem;
		border: 0;
		background: transparent;
		color: var(--pico-muted-color);
		font-size: 1.4rem;
	}

	.repository-summary {
		display: flex;
		align-items: flex-end;
		justify-content: space-between;
		gap: 1rem;
		margin-bottom: 1rem;
		color: var(--pico-muted-color);
	}

	.repository-summary h2 {
		margin: 0 0 0.2rem;
		font-size: 1.2rem;
	}

	.repository-summary p {
		margin: 0;
		font-size: 0.82rem;
	}

	.repository-summary strong {
		color: var(--pico-color);
	}

	.repository-grid {
		display: grid;
		gap: 1rem;
		grid-template-columns: repeat(2, minmax(0, 1fr));
	}

	.state-card,
	.empty-state {
		border: 1px solid var(--pico-muted-border-color);
		border-radius: var(--pico-border-radius);
		background: var(--pico-card-background-color);
		box-shadow: 0 1.4rem 4rem color-mix(in srgb, var(--pico-color) 5%, transparent);
	}

	.error-state,
	.empty-state {
		display: flex;
		align-items: center;
		gap: 1rem;
		padding: 1.5rem;
	}

	.state-icon {
		display: grid;
		width: 2.5rem;
		height: 2.5rem;
		flex: 0 0 auto;
		place-items: center;
		border-radius: 50%;
		background: color-mix(in srgb, var(--pico-del-color) 12%, transparent);
		color: var(--pico-del-color);
		font-weight: 800;
	}

	.error-state h2,
	.empty-state h3 {
		margin: 0 0 0.25rem;
		font-size: 1.1rem;
	}

	.error-state p,
	.empty-state p {
		margin: 0 0 0.25rem;
		color: var(--pico-muted-color);
	}

	.empty-illustration {
		display: grid;
		width: 3.5rem;
		height: 3.5rem;
		flex: 0 0 auto;
		place-items: center;
		border-radius: 0.9rem;
		background: color-mix(in srgb, var(--pico-primary) 9%, transparent);
		color: var(--pico-primary);
	}

	.empty-illustration svg {
		width: 2.75rem;
		fill: none;
		stroke: currentColor;
		stroke-linecap: round;
		stroke-linejoin: round;
		stroke-width: 3;
	}

	@media (max-width: 700px) {
		.repository-summary {
			align-items: flex-start;
			flex-direction: column;
			gap: 0.25rem;
		}

		.repository-grid {
			grid-template-columns: 1fr;
		}
	}
</style>
