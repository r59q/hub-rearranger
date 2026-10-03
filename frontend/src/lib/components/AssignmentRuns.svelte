<script lang="ts">
	import type { IssueAssignment } from '$lib/server/agents-assignment-response';
	let { requests }: { requests: IssueAssignment['requests'] } = $props();
	const labels = {
		requested: 'Request posted',
		queued: 'Workflow waiting',
		running: 'Workflow running',
		blocked: 'Execution blocked',
		failed: 'Workflow failed',
		cancelled: 'Workflow cancelled',
		completed: 'Workflow completed',
		proposal: 'Patch proposal',
		unavailable: 'Evidence unavailable'
	};
</script>

<section aria-labelledby="assignment-history">
	<h2 id="assignment-history">Assignment activity</h2>
	<p>
		Fresh from GitHub. Refresh this page to update; workflow completion does not imply passing
		checks.
	</p>
	{#if requests.length === 0}<p>No assignment request is recorded on this issue.</p>
	{:else}{#each requests as request (request.comment_id)}
			<article>
				<h3>{labels[request.state]}</h3>
				<p>{request.summary}</p>
				<p>
					{request.requester} ·
					<time datetime={request.created_at}>{new Date(request.created_at).toLocaleString()}</time>
				</p>
				<p><code>codex-thorough@{request.profile_revision}</code></p>
				{#if request.edited}<p>
						This comment was edited. Edits do not start or change an assignment; inspect its
						original workflow.
					</p>{/if}
				<nav aria-label="Assignment sources">
					<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
					<a href={request.url} target="_blank" rel="noreferrer">Request comment</a>
					{#if request.run}
						<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
						<a href={request.run.url} target="_blank" rel="noreferrer"
							>Workflow · attempt {request.run.attempt}</a
						>
					{/if}
				</nav>
				{#if request.proposal}<div class="proposal">
						<p>
							<strong
								>{request.proposal.state === 'closed'
									? 'Closed PR'
									: request.proposal.draft
										? 'Draft PR'
										: 'Open PR'}</strong
							>
							<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
							<a href={request.proposal.url} target="_blank" rel="noreferrer"
								>#{request.proposal.number}</a
							>
						</p>
						<p>
							Branch:
							<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
							<a href={request.proposal.branch_url} target="_blank" rel="noreferrer"
								><code>{request.proposal.branch}</code></a
							>
						</p>
						{#if request.proposal.check}<p>
								{request.proposal.check.summary}
								<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
								<a href={request.proposal.check.url} target="_blank" rel="noreferrer"
									>Check · {request.proposal.check.conclusion}</a
								>
							</p>
						{:else}<p role="status">
								Publication check is missing. Inspect the original workflow before retrying.
							</p>{/if}
					</div>{/if}
			</article>
		{/each}{/if}
	{#if requests.length === 10}<p>
			Showing the ten newest requests. Open the issue on GitHub for older history.
		</p>{/if}
</section>

<style>
	article {
		margin-block: 1rem;
		padding: 1.25rem;
	}
	h3 {
		font-size: 1.1rem;
	}
	nav {
		display: flex;
		flex-wrap: wrap;
		justify-content: flex-start;
		gap: 1rem;
	}
	code,
	a {
		overflow-wrap: anywhere;
	}
	p {
		margin-block: 0.6rem;
	}
	.proposal {
		border-top: 1px solid var(--pico-muted-border-color);
		margin-top: 1rem;
		padding-top: 0.5rem;
	}
</style>
