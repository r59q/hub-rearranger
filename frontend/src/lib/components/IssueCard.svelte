<script lang="ts">
	import { formatIssueActivity } from '$lib/issues/presentation';
	import type { Issue, RelatedIssue } from '$lib/issues/types';

	let { issue, expanded = false }: { issue: Issue; expanded?: boolean } = $props();

	function relationName(relation: RelatedIssue): string {
		return `${relation.repository}#${relation.number}`;
	}
</script>

<article class:expanded>
	<header>
		<div class="identity">
			<span class="state-dot" aria-hidden="true"></span>
			<span>{issue.repository} <strong>#{issue.number}</strong></span>
		</div>
		<time datetime={issue.updated_at}>{formatIssueActivity(issue.updated_at)}</time>
	</header>

	<div class="issue-copy">
		<h3>
			<!-- GitHub URLs are API data, not application routes. -->
			<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
			<a href={issue.html_url} target="_blank" rel="noreferrer">{issue.title}</a>
		</h3>
		<p>{issue.description || 'No description provided on GitHub.'}</p>
	</div>

	{#if issue.labels.length > 0}
		<ul class="labels" aria-label="Labels">
			{#each issue.labels as label (label.name)}
				<li>{label.name}</li>
			{/each}
		</ul>
	{/if}

	<div class="relationship-counts" aria-label="Issue relationships">
		<span><strong>{issue.subtasks.length}</strong> subtasks</span>
		<span><strong>{issue.linked_issues.length}</strong> linked issues</span>
		<span><strong>{issue.linked_pull_requests.length}</strong> pull requests</span>
	</div>

	{#if expanded}
		{#if issue.details_available}
			<div class="relationships">
				<div>
					<h4>Subtasks</h4>
					{#if issue.subtasks.length > 0}
						<ul>
							{#each issue.subtasks as subtask (subtask.html_url)}
								<li>
									<span
										class:closed={subtask.state === 'closed'}
										class="relation-state"
										aria-label={subtask.state}
									></span>
									<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
									<a href={subtask.html_url} target="_blank" rel="noreferrer"
										>{relationName(subtask)} · {subtask.title}</a
									>
								</li>
							{/each}
						</ul>
					{:else}
						<p>None</p>
					{/if}
				</div>
				<div>
					<h4>Linked work</h4>
					{#if issue.linked_issues.length + issue.linked_pull_requests.length > 0}
						<ul>
							{#each issue.linked_issues as linkedIssue (linkedIssue.html_url)}
								<li>
									<span class="relation-kind">Issue</span>
									<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
									<a href={linkedIssue.html_url} target="_blank" rel="noreferrer"
										>{relationName(linkedIssue)} · {linkedIssue.title}</a
									>
								</li>
							{/each}
							{#each issue.linked_pull_requests as pullRequest (pullRequest.html_url)}
								<li>
									<span class="relation-kind pull-request">PR</span>
									<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
									<a href={pullRequest.html_url} target="_blank" rel="noreferrer"
										>{relationName(pullRequest)} · {pullRequest.title}</a
									>
								</li>
							{/each}
						</ul>
					{:else}
						<p>None</p>
					{/if}
				</div>
			</div>
		{:else}
			<p class="details-unavailable" role="status">
				Relationship details are temporarily unavailable.
			</p>
		{/if}
	{/if}
</article>

<style>
	article {
		display: flex;
		min-width: 0;
		flex-direction: column;
		gap: 1rem;
		margin: 0;
		padding: 1.25rem;
		border: 1px solid var(--pico-muted-border-color);
		box-shadow: 0 0.7rem 2rem color-mix(in srgb, var(--pico-color) 4%, transparent);
	}

	article.expanded {
		padding: clamp(1.1rem, 3vw, 1.5rem);
	}

	header,
	.identity,
	.relationship-counts,
	.relationships li {
		display: flex;
		align-items: center;
	}

	header {
		justify-content: space-between;
		gap: 1rem;
		color: var(--pico-muted-color);
		font-size: 0.72rem;
	}

	.identity {
		min-width: 0;
		gap: 0.4rem;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.identity strong {
		color: var(--pico-color);
	}

	.state-dot,
	.relation-state {
		width: 0.55rem;
		height: 0.55rem;
		flex: 0 0 auto;
		border: 2px solid var(--pico-primary);
		border-radius: 50%;
	}

	.relation-state.closed {
		border-color: #8250df;
		background: #8250df;
	}

	time {
		flex: 0 0 auto;
	}

	h3 {
		margin: 0 0 0.5rem;
		font-size: 1.05rem;
		line-height: 1.35;
	}

	h3 a {
		color: var(--pico-color);
		text-decoration: none;
	}

	h3 a:hover {
		color: var(--pico-primary);
	}

	.issue-copy p {
		display: -webkit-box;
		overflow: hidden;
		margin: 0;
		-webkit-box-orient: vertical;
		-webkit-line-clamp: 2;
		line-clamp: 2;
		color: var(--pico-muted-color);
		font-size: 0.84rem;
		white-space: pre-line;
	}

	.expanded .issue-copy p {
		-webkit-line-clamp: 4;
		line-clamp: 4;
	}

	.labels,
	.relationships ul {
		margin: 0;
		padding: 0;
		list-style: none;
	}

	.labels {
		display: flex;
		flex-wrap: wrap;
		gap: 0.35rem;
	}

	.labels li,
	.relation-kind {
		padding: 0.15rem 0.45rem;
		border: 1px solid color-mix(in srgb, var(--pico-primary) 28%, var(--pico-muted-border-color));
		border-radius: 2rem;
		background: color-mix(in srgb, var(--pico-primary) 7%, transparent);
		font-size: 0.65rem;
		font-weight: 650;
	}

	.relationship-counts {
		flex-wrap: wrap;
		gap: 0.55rem 1rem;
		padding-top: 0.8rem;
		border-top: 1px solid var(--pico-muted-border-color);
		color: var(--pico-muted-color);
		font-size: 0.72rem;
	}

	.relationship-counts strong {
		color: var(--pico-color);
	}

	.relationships {
		display: grid;
		gap: 1.25rem;
		grid-template-columns: repeat(2, minmax(0, 1fr));
		padding-top: 0.25rem;
	}

	.relationships h4 {
		margin: 0 0 0.55rem;
		font-size: 0.78rem;
		text-transform: uppercase;
		letter-spacing: 0.06em;
	}

	.relationships li {
		min-width: 0;
		gap: 0.45rem;
		margin-top: 0.45rem;
		font-size: 0.74rem;
	}

	.relationships a {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.relationships p,
	.details-unavailable {
		margin: 0;
		color: var(--pico-muted-color);
		font-size: 0.75rem;
	}

	.relation-kind {
		flex: 0 0 auto;
		color: var(--pico-primary);
	}

	.relation-kind.pull-request {
		border-color: color-mix(in srgb, #8250df 35%, var(--pico-muted-border-color));
		background: color-mix(in srgb, #8250df 8%, transparent);
		color: #8250df;
	}

	@media (max-width: 700px) {
		header {
			align-items: flex-start;
			flex-direction: column;
			gap: 0.35rem;
		}

		.relationships {
			grid-template-columns: 1fr;
		}
	}
</style>
