<script lang="ts">
	import { resolve } from '$app/paths';
	import { issuePageHref } from '$lib/issues/presentation';
	import type { IssuePage } from '$lib/issues/types';
	import IssueCard from './IssueCard.svelte';

	let {
		issuePage,
		issueError,
		selectedRepositoryCount,
		variant = 'dashboard'
	}: {
		issuePage: IssuePage;
		issueError: string | null;
		selectedRepositoryCount: number;
		variant?: 'dashboard' | 'workdesk';
	} = $props();

	let fullPage = $derived(variant === 'workdesk');
</script>

<section class:full-page={fullPage} class="issues-section" aria-labelledby="issues-title">
	<div class="section-heading">
		<div>
			<p class="eyebrow">{fullPage ? 'Work page' : 'Recent activity'}</p>
			{#if fullPage}
				<h1 id="issues-title">Issues</h1>
			{:else}
				<h2 id="issues-title">Issues</h2>
			{/if}
			<p>
				{fullPage
					? 'Review open work across your selected repositories, newest activity first.'
					: 'Open work across your selected repositories, newest activity first.'}
			</p>
		</div>
		{#if !fullPage && !issueError && selectedRepositoryCount > 0 && issuePage.issues.length > 0}
			<a class="browse-link secondary outline" href={resolve('/issues')}>
				Browse all issues
				<svg viewBox="0 0 24 24" aria-hidden="true"><path d="m9 6 6 6-6 6" /></svg>
			</a>
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
		<div class:full-list={fullPage} class="issues-view">
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
					<IssueCard {issue} expanded={fullPage} />
				{/each}
			</div>

			{#if fullPage}
				<nav class="pagination" aria-label="Issue pages">
					<!-- Pagination adds a query string to the base-path-aware resolved issues route. -->
					<!-- eslint-disable svelte/no-navigation-without-resolve -->
					{#if issuePage.page > 1}
						<a
							class="secondary outline"
							href={resolve('/issues') + issuePageHref(issuePage.page - 1)}>← Previous</a
						>
					{:else}
						<span></span>
					{/if}
					<span>Page <strong>{issuePage.page}</strong></span>
					{#if issuePage.has_next}
						<a
							class="secondary outline"
							href={resolve('/issues') + issuePageHref(issuePage.page + 1)}>Next →</a
						>
					{:else}
						<span></span>
					{/if}
					<!-- eslint-enable svelte/no-navigation-without-resolve -->
				</nav>
			{/if}
		</div>
	{/if}
</section>

<style>
	.issues-section {
		margin-top: clamp(3rem, 7vw, 5rem);
	}

	.issues-section.full-page {
		margin-top: 0;
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

	h1,
	h2 {
		margin: 0;
		letter-spacing: -0.025em;
	}

	h1 {
		font-size: clamp(2.35rem, 5vw, 3.75rem);
		letter-spacing: -0.045em;
		line-height: 1.05;
	}

	h2 {
		font-size: 1.55rem;
	}

	.section-heading p:last-child {
		margin: 0.3rem 0 0;
		color: var(--pico-muted-color);
		font-size: 0.84rem;
	}

	.browse-link {
		display: inline-flex;
		width: auto;
		flex: 0 0 auto;
		align-items: center;
		gap: 0.4rem;
		margin: 0;
		padding: 0.55rem 0.8rem;
		font-size: 0.78rem;
		text-decoration: none;
	}

	.browse-link svg {
		width: 1rem;
		fill: none;
		stroke: currentColor;
		stroke-linecap: round;
		stroke-linejoin: round;
		stroke-width: 2;
	}

	.issue-grid {
		display: grid;
		gap: 0.85rem;
		grid-template-columns: repeat(2, minmax(0, 1fr));
	}

	.full-list .issue-grid {
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
