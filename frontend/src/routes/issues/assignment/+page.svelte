<script lang="ts">
	import { resolve } from '$app/paths';
	import { enhance } from '$app/forms';
	import AssignmentRuns from '$lib/components/AssignmentRuns.svelte';
	import type { PageProps } from './$types';
	let { data, form }: PageProps = $props();
	let pending = $state(false);
	const issueQuery = $derived(
		new URLSearchParams({
			repository: data.repository?.full_name ?? '',
			number: String(data.assignment?.number ?? '')
		}).toString()
	);
	const submission = () => {
		pending = true;
		return async ({ update }: { update: () => Promise<void> }) => {
			try {
				await update();
			} finally {
				pending = false;
			}
		};
	};
</script>

<svelte:head><title>Issue assignment · Hub Rearranger</title></svelte:head>
<main class="container">
	<header>
		<a href={resolve('/issues')}>← Issues</a>
		<h1>{data.assignment?.title ?? 'Issue assignment'}</h1>
		{#if data.assignment}<p>
				{data.assignment.repository} #{data.assignment.number} · {data.assignment.issue_state}
			</p>
			<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
			<a href={data.assignment.url} target="_blank" rel="noreferrer">Open issue on GitHub</a>{/if}
	</header>
	{#if data.error}<p role="alert">{data.error}</p>{/if}
	{#if form?.error}<p role="alert" class="notice">{form.error}</p>{/if}
	{#if data.assignment}
		<details>
			<summary>Issue description</summary>
			<p class="description">{data.assignment.body || 'No description provided on GitHub.'}</p>
		</details>
		<section aria-labelledby="assign-heading">
			<h2 id="assign-heading">Assign Codex Thorough</h2>
			<p>
				Codex can read repository context, propose a patch on a dedicated branch, open a draft PR,
				and report checks. It uses gpt-6.1-sol with high reasoning. Its workload runs in an offline
				workspace; it cannot merge or release. You remain responsible for review.
			</p>
			{#if form?.reconnect || data.identity.state === 'signed_out' || data.identity.state === 'reconnect_required'}
				<p>
					Connect your GitHub account to post the request as yourself. Assignment requires maintain
					or admin access.
				</p>
				<a href={resolve('/account')}>Connect GitHub account</a>
			{:else if data.identity.state !== 'authenticated'}
				<p>
					GitHub sign-in is {data.identity.state === 'disabled'
						? 'not configured'
						: 'temporarily unavailable'}. Assignment activity remains readable.
				</p>
				<a href={resolve('/account')}>Account connection</a>
			{:else if form?.review && data.assignment.assignable}
				<p>
					This will post one new comment on <strong
						>{form.review.repository} #{form.review.number}</strong
					>
					as <strong>{form.review.user.login}</strong>. The workflow independently verifies access,
					policy and runner readiness. Posting a request does not guarantee execution.
				</p>
				<pre><code>{form.review.command}</code></pre>
				<form
					method="POST"
					action={resolve(`/issues/assignment?/assign&${issueQuery}`)}
					use:enhance={submission}
				>
					<input type="hidden" name="csrf" value={data.identity.csrf ?? ''} /><input
						type="hidden"
						name="review_token"
						value={form.review.review_token}
					/>
					<label
						><input type="checkbox" name="confirm" value="reviewed" required disabled={pending} /> I approve
						posting this assignment request with branch and draft PR authority.</label
					>
					<button type="submit" disabled={pending} aria-busy={pending}
						>{pending ? 'Posting request…' : 'Post assignment request'}</button
					>
				</form>
				<p class="muted">
					Review expires in ten minutes. If a response is lost, inspect GitHub before creating
					another request.
				</p>
			{:else if data.assignment.assignable}
				<p>{data.assignment.reason}</p>
				<form
					method="POST"
					action={resolve(`/issues/assignment?/review&${issueQuery}`)}
					use:enhance={submission}
				>
					<input type="hidden" name="csrf" value={data.identity.csrf ?? ''} /><input
						type="hidden"
						name="profile_revision"
						value={data.assignment.profile_revision}
					/>
					<button type="submit" disabled={pending} aria-busy={pending}
						>{pending ? 'Checking access…' : 'Review assignment'}</button
					>
				</form>
			{:else}<p>{data.assignment.reason}</p>{/if}
		</section>
		<AssignmentRuns requests={data.assignment.requests} />{/if}
	<a href={resolve(`/issues/assignment?${issueQuery}`)}>Refresh GitHub status</a>
</main>

<style>
	main {
		padding-block: 2rem 4rem;
		max-width: 58rem;
	}
	header {
		margin-bottom: 2rem;
	}
	h1 {
		font-size: clamp(1.8rem, 5vw, 2.6rem);
		overflow-wrap: anywhere;
		margin-block: 1rem 0.5rem;
	}
	section {
		margin-block: 2rem;
	}
	pre {
		white-space: pre-wrap;
		overflow-wrap: anywhere;
		padding: 1rem;
	}
	.description {
		white-space: pre-wrap;
		overflow-wrap: anywhere;
	}
	.notice {
		padding: 1rem;
		border: 1px solid var(--pico-del-color);
		border-radius: var(--pico-border-radius);
	}
	.muted {
		color: var(--pico-muted-color);
	}
	button {
		width: auto;
		max-width: 100%;
	}
	@media (max-width: 480px) {
		button {
			width: 100%;
		}
	}
</style>
