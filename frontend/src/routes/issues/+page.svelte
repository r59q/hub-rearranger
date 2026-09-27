<script lang="ts">
	import { resolve } from '$app/paths';
	import IssuesPanel from '$lib/components/IssuesPanel.svelte';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();
</script>

<svelte:head>
	<title>Issues · Hub Rearranger</title>
	<meta
		name="description"
		content="Review open GitHub issues across the repositories selected in your workdesk."
	/>
</svelte:head>

<main class="container">
	{#if data.serviceError}
		<section class="error-state" aria-labelledby="service-error-title">
			<span class="state-icon" aria-hidden="true">!</span>
			<div>
				<p class="eyebrow">Work page</p>
				<h1 id="service-error-title">Issues are unavailable</h1>
				<p>{data.serviceError}</p>
				<a href={resolve('/issues')}>Try again</a>
			</div>
		</section>
	{:else}
		<IssuesPanel
			issuePage={data.issuePage}
			issueError={data.issueError}
			selectedRepositoryCount={data.repositories.length}
			variant="workdesk"
		/>
	{/if}
</main>

<style>
	main {
		width: 100%;
		padding-block: clamp(2.25rem, 5vw, 4.5rem) 5rem;
	}

	.error-state {
		display: flex;
		align-items: center;
		gap: 1rem;
		padding: 1.5rem;
		border: 1px solid var(--pico-muted-border-color);
		border-radius: var(--pico-border-radius);
		background: var(--pico-card-background-color);
		box-shadow: 0 1.4rem 4rem color-mix(in srgb, var(--pico-color) 5%, transparent);
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

	.eyebrow {
		margin: 0 0 0.5rem;
		color: var(--pico-primary);
		font-size: 0.67rem;
		font-weight: 750;
		letter-spacing: 0.13em;
		text-transform: uppercase;
	}

	h1 {
		margin: 0;
		font-size: clamp(2rem, 5vw, 3rem);
		letter-spacing: -0.04em;
	}

	.error-state div > p:last-of-type {
		margin: 0.4rem 0;
		color: var(--pico-muted-color);
	}
</style>
