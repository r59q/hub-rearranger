<script lang="ts">
	import { resolve } from '$app/paths';
	import { issuePageHref } from '$lib/issues/presentation';
	import type { IssuePage } from '$lib/issues/types';
	import IssueCard from './IssueCard.svelte';

	let {
		issuePage,
		issueError,
		initiallyExpanded,
		selectedRepositoryCount
	}: {
		issuePage: IssuePage;
		issueError: string | null;
		initiallyExpanded: boolean;
		selectedRepositoryCount: number;
	} = $props();

	let expanded = $state(false);
	$effect(() => {
		if (initiallyExpanded) {
			expanded = true;
		}
	});
</script>

<section class="issues-section" aria-labelledby="issues-title">
	<div class="section-heading">
		<div>
			<p class="eyebrow">Recent activity</p>
			<h2 id="issues-title">Issues</h2>
			<p>Open work across your selected repositories, newest activity first.</p>
		</div>
		{#if !issueError && selectedRepositoryCount > 0 && issuePage.issues.length > 0}
			<button
				type="button"
				class="accordion-trigger secondary outline"
				aria-expanded={expanded}
				aria-controls="issues-view"
				onclick={() => (expanded = !expanded)}
			>
				{expanded ? 'Collapse issues' : 'Browse all issues'}
				<svg class:expanded viewBox="0 0 24 24" aria-hidden="true"><path d="m7 10 5 5 5-5" /></svg>
			</button>
		{/if}
	</div>

	{#if selectedRepositoryCount === 0}
		<div class="issue-state">
			<span class="state-icon" aria-hidden="true">#</span>
			<div>
				<h3>Select a repository to see issues</h3>
				<p>Issues from repositories in your workspace will appear here.</p>
			</div>
		</div>
	{:else if issueError}
		<div class="issue-state error" role="status">
			<span class="state-icon" aria-hidden="true">!</span>
			<div>
				<h3>Issues are unavailable</h3>
				<p>{issueError}</p>
			</div>
		</div>
	{:else if issuePage.issues.length === 0}
		<div class="issue-state">
			<span class="state-icon" aria-hidden="true">✓</span>
			<div>
				<h3>No open issues on this page</h3>
				<p>
					{issuePage.page === 1
						? 'Your selected repositories have no open issues.'
						: 'Go back to an earlier page to continue browsing.'}
				</p>
			</div>
		</div>
	{:else}
		<div id="issues-view" class:paginated={expanded} class="issues-view">
			{#if issuePage.unavailable_repositories.length > 0}
				<p class="partial-notice" role="status">
					Some issues could not be loaded from {issuePage.unavailable_repositories.join(', ')}.
				</p>
			{/if}
			{#if issuePage.incomplete_details > 0}
				<p class="partial-notice" role="status">
					Relationship details are unavailable for {issuePage.incomplete_details}
					{issuePage.incomplete_details === 1 ? 'issue' : 'issues'}.
				</p>
			{/if}

			<div class="issue-grid">
				{#each issuePage.issues as issue (issue.id)}
					<IssueCard {issue} {expanded} />
				{/each}
			</div>

			{#if expanded}
				<nav class="pagination" aria-label="Issue pages">
					{#if issuePage.page > 1}
						<!-- Query strings are appended to the base-path-aware resolved home route. -->
						<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
						<a class="secondary outline" href={resolve('/') + issuePageHref(issuePage.page - 1)}
							>← Previous</a
						>
					{:else}
						<span></span>
					{/if}
					<span>Page <strong>{issuePage.page}</strong></span>
					{#if issuePage.has_next}
						<!-- Query strings are appended to the base-path-aware resolved home route. -->
						<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
						<a class="secondary outline" href={resolve('/') + issuePageHref(issuePage.page + 1)}
							>Next →</a
						>
					{:else}
						<span></span>
					{/if}
				</nav>
			{/if}
		</div>
	{/if}
</section>

<style>
	.issues-section {
		margin-top: clamp(3rem, 7vw, 5rem);
	}

	.section-heading {
		display: flex;
		align-items: flex-end;
		justify-content: space-between;
		gap: 1.5rem;
		margin-bottom: 1.15rem;
	}

	.eyebrow {
		margin: 0 0 0.35rem;
		color: var(--pico-primary);
		font-size: 0.67rem;
		font-weight: 750;
		letter-spacing: 0.13em;
		text-transform: uppercase;
	}

	h2 {
		margin: 0;
		font-size: 1.55rem;
		letter-spacing: -0.025em;
	}

	.section-heading p:last-child {
		margin: 0.3rem 0 0;
		color: var(--pico-muted-color);
		font-size: 0.84rem;
	}

	.accordion-trigger {
		display: inline-flex;
		width: auto;
		flex: 0 0 auto;
		align-items: center;
		gap: 0.4rem;
		margin: 0;
		padding: 0.55rem 0.8rem;
		font-size: 0.78rem;
	}

	.accordion-trigger svg {
		width: 1rem;
		fill: none;
		stroke: currentColor;
		stroke-linecap: round;
		stroke-linejoin: round;
		stroke-width: 2;
		transition: transform 160ms ease;
	}

	.accordion-trigger svg.expanded {
		transform: rotate(180deg);
	}

	.issue-grid {
		display: grid;
		gap: 0.85rem;
		grid-template-columns: repeat(2, minmax(0, 1fr));
	}

	.paginated .issue-grid {
		grid-template-columns: 1fr;
	}

	.partial-notice {
		margin: 0 0 0.75rem;
		padding: 0.65rem 0.8rem;
		border: 1px solid color-mix(in srgb, #bf8700 35%, var(--pico-muted-border-color));
		border-radius: var(--pico-border-radius);
		background: color-mix(in srgb, #bf8700 7%, transparent);
		font-size: 0.76rem;
	}

	.issue-state {
		display: flex;
		align-items: center;
		gap: 1rem;
		padding: 1.35rem;
		border: 1px solid var(--pico-muted-border-color);
		border-radius: var(--pico-border-radius);
		background: var(--pico-card-background-color);
	}

	.state-icon {
		display: grid;
		width: 2.4rem;
		height: 2.4rem;
		flex: 0 0 auto;
		place-items: center;
		border-radius: 50%;
		background: color-mix(in srgb, var(--pico-primary) 10%, transparent);
		color: var(--pico-primary);
		font-weight: 750;
	}

	.issue-state.error .state-icon {
		background: color-mix(in srgb, var(--pico-del-color) 10%, transparent);
		color: var(--pico-del-color);
	}

	.issue-state h3,
	.issue-state p {
		margin: 0;
	}

	.issue-state h3 {
		font-size: 1rem;
	}

	.issue-state p {
		margin-top: 0.2rem;
		color: var(--pico-muted-color);
		font-size: 0.82rem;
	}

	.pagination {
		display: grid;
		align-items: center;
		grid-template-columns: 1fr auto 1fr;
		margin-top: 1rem;
	}

	.pagination > :last-child {
		justify-self: end;
	}

	.pagination a {
		padding: 0.5rem 0.75rem;
		font-size: 0.76rem;
		text-decoration: none;
	}

	.pagination > span {
		color: var(--pico-muted-color);
		font-size: 0.76rem;
	}

	@media (prefers-reduced-motion: reduce) {
		.accordion-trigger svg {
			transition: none;
		}
	}

	@media (max-width: 700px) {
		.section-heading {
			align-items: flex-start;
			flex-direction: column;
			gap: 0.85rem;
		}

		.issue-grid {
			grid-template-columns: 1fr;
		}

		.pagination a {
			padding-inline: 0.55rem;
		}
	}
</style>
