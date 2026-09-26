<script lang="ts">
	import { enhance } from '$app/forms';
	import { resolve } from '$app/paths';
	import type { SubmitFunction } from '@sveltejs/kit';
	import type { PageProps } from './$types';

	let { data, form }: PageProps = $props();
	let repositoryDialog: HTMLDialogElement;
	let saving = $state(false);

	const enhanceSelection: SubmitFunction = () => {
		saving = true;
		return async ({ result, update }) => {
			await update();
			saving = false;
			if (result.type === 'success') repositoryDialog.close();
		};
	};
</script>

<svelte:head>
	<title>Repositories · Hub Rearranger</title>
	<meta name="description" content="Choose the GitHub repositories in your focused workspace." />
</svelte:head>

<div class="site-shell">
	<header class="app-header">
		<div class="container header-inner">
			<a class="brand" href={resolve('/')} aria-label="Hub Rearranger home">
				<span class="brand-mark" aria-hidden="true">H</span><span>Hub Rearranger</span>
			</a>
			<nav aria-label="Primary navigation">
				<a class="active" href={resolve('/')} aria-current="page">
					<svg viewBox="0 0 24 24" aria-hidden="true"
						><path
							d="M4 5.5A2.5 2.5 0 0 1 6.5 3h11A2.5 2.5 0 0 1 20 5.5v13a2.5 2.5 0 0 1-2.5 2.5h-11A2.5 2.5 0 0 1 4 18.5v-13ZM8 8h8m-8 4h8m-8 4h5"
						/></svg
					>
					Repositories
				</a>
			</nav>
			<div class="account-mark" aria-label="GitHub account configured">
				<svg viewBox="0 0 24 24" aria-hidden="true"
					><path
						d="M12 2a10 10 0 0 0-3.16 19.49c.5.1.68-.22.68-.48v-1.86c-2.78.6-3.37-1.18-3.37-1.18-.45-1.16-1.11-1.47-1.11-1.47-.91-.62.07-.61.07-.61 1 .07 1.53 1.03 1.53 1.03.9 1.53 2.35 1.09 2.92.83.09-.65.35-1.09.64-1.34-2.22-.25-4.56-1.11-4.56-4.94 0-1.09.39-1.98 1.03-2.68-.1-.25-.45-1.27.1-2.64 0 0 .84-.27 2.75 1.02A9.56 9.56 0 0 1 12 6.83a9.5 9.5 0 0 1 2.5.34c1.91-1.29 2.75-1.02 2.75-1.02.55 1.37.2 2.39.1 2.64.64.7 1.03 1.59 1.03 2.68 0 3.84-2.34 4.68-4.57 4.93.36.31.68.92.68 1.85v2.76c0 .27.18.58.69.48A10 10 0 0 0 12 2Z"
					/></svg
				>
			</div>
		</div>
	</header>

	<main class="container">
		<section class="page-heading" aria-labelledby="page-title">
			<div>
				<p class="eyebrow">Workspace</p>
				<h1 id="page-title">Repositories</h1>
				<p>Keep the repositories that matter close at hand.</p>
			</div>
			<button
				class="manage-button"
				type="button"
				onclick={() => repositoryDialog.showModal()}
				disabled={data.serviceError !== null}
			>
				<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 5v14M5 12h14" /></svg>
				Manage repositories
			</button>
		</section>

		{#if form?.message}
			<div class:success={form.success} class="notice" role={form.success ? 'status' : 'alert'}>
				<span>{form.message}</span>
				<button
					type="button"
					class="icon-button"
					aria-label="Dismiss message"
					onclick={() => (form = null)}>×</button
				>
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
		{:else if data.repositories.length === 0}
			<section class="state-card empty-state" aria-labelledby="empty-title">
				<div class="empty-illustration" aria-hidden="true">
					<svg viewBox="0 0 96 96"
						><path
							d="M20 29.5A9.5 9.5 0 0 1 29.5 20h37A9.5 9.5 0 0 1 76 29.5V76H29.5a9.5 9.5 0 0 1-9.5-9.5v-37Z"
						/><path d="M20 65.5a9.5 9.5 0 0 1 9.5-9.5H76M32 33h27M32 44h18" /></svg
					>
				</div>
				<h2 id="empty-title">Build your workspace</h2>
				<p>Select repositories from your GitHub account to see them here.</p>
				<button type="button" onclick={() => repositoryDialog.showModal()}
					>Choose repositories</button
				>
			</section>
		{:else}
			<div class="repository-summary">
				<p>
					<strong>{data.repositories.length}</strong>
					{data.repositories.length === 1 ? 'repository' : 'repositories'} in this workspace
				</p>
				<small>Repository details stay in sync with GitHub.</small>
			</div>
			<section class="repository-grid" aria-label="Selected repositories">
				{#each data.repositories as repository (repository.id)}
					<article class="repository-card">
						<header>
							<div class="repo-icon" aria-hidden="true">
								<svg viewBox="0 0 24 24"
									><path
										d="M5 3.75h11.25A2.75 2.75 0 0 1 19 6.5v12.75H7.75A2.75 2.75 0 0 1 5 16.5V3.75Zm0 12.75a2.75 2.75 0 0 1 2.75-2.75H19M8.5 7.5h7"
									/></svg
								>
							</div>
							<span class="visibility">{repository.private ? 'Private' : 'Public'}</span>
						</header>
						<div class="repo-copy">
							<small>{repository.owner}</small>
							<h2>{repository.name}</h2>
							<p>{repository.description || 'No description provided on GitHub.'}</p>
						</div>
						<footer>
							<span class="branch" title="Default branch"
								><svg viewBox="0 0 24 24" aria-hidden="true"
									><path d="M6 3v12a3 3 0 0 0 3 3h12M16 14l4 4-4 4M18 9V3" /></svg
								>{repository.default_branch || 'No default branch'}</span
							>
							<!-- GitHub URLs are API data, not application routes. -->
							<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
							<a href={repository.html_url} target="_blank" rel="noreferrer"
								>Open on GitHub <svg viewBox="0 0 24 24" aria-hidden="true"
									><path d="M14 5h5v5M19 5l-8 8M19 14v5H5V5h5" /></svg
								></a
							>
						</footer>
					</article>
				{/each}
			</section>
		{/if}
	</main>

	<footer class="site-footer">
		<div class="container">
			<span><span class="sync-dot" aria-hidden="true"></span>GitHub is the source of truth</span
			><span>Hub Rearranger</span>
		</div>
	</footer>
</div>

<dialog bind:this={repositoryDialog} aria-labelledby="dialog-title">
	<article>
		<header class="dialog-header">
			<div>
				<p class="eyebrow">GitHub account</p>
				<h2 id="dialog-title">Manage repositories</h2>
				<p>Select the repositories to keep in this workspace.</p>
				<small>This does not make changes on GitHub.</small>
			</div>
			<button
				type="button"
				class="icon-button"
				aria-label="Close"
				onclick={() => repositoryDialog.close()}>×</button
			>
		</header>
		<form method="POST" use:enhance={enhanceSelection}>
			<fieldset disabled={saving}>
				<legend class="sr-only">Available repositories</legend>
				{#if data.availableRepositories.length === 0}
					<p class="dialog-empty">
						No repositories are available to the configured GitHub account.
					</p>
				{:else}
					<div class="repository-options">
						{#each data.availableRepositories as repository (repository.id)}
							<label class="repository-option">
								<input
									type="checkbox"
									name="repositoryId"
									value={repository.id}
									checked={repository.selected}
								/>
								<span
									><strong>{repository.full_name}</strong><small
										>{repository.private ? 'Private' : 'Public'} · {repository.description ||
											'No description'}</small
									></span
								>
							</label>
						{/each}
					</div>
				{/if}
			</fieldset>
			<footer class="dialog-actions">
				<button type="button" class="secondary" onclick={() => repositoryDialog.close()}
					>Cancel</button
				>
				<button type="submit" aria-busy={saving} disabled={saving}
					>{saving ? 'Saving…' : 'Save selection'}</button
				>
			</footer>
		</form>
	</article>
</dialog>

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
	.app-header {
		border-bottom: 1px solid var(--pico-muted-border-color);
		background: color-mix(in srgb, var(--pico-background-color) 94%, transparent);
		backdrop-filter: blur(0.75rem);
	}
	.header-inner {
		display: grid;
		min-height: 4.5rem;
		align-items: center;
		grid-template-columns: 1fr auto 1fr;
	}
	.brand {
		display: inline-flex;
		width: max-content;
		align-items: center;
		gap: 0.7rem;
		color: var(--pico-color);
		font-weight: 750;
		letter-spacing: -0.02em;
		text-decoration: none;
	}
	.brand-mark {
		display: grid;
		width: 2.15rem;
		height: 2.15rem;
		place-items: center;
		border-radius: 0.6rem;
		background: var(--pico-primary-background);
		color: var(--pico-primary-inverse);
		font-weight: 800;
		box-shadow: 0 0.4rem 1rem color-mix(in srgb, var(--pico-primary) 20%, transparent);
	}
	nav a {
		display: inline-flex;
		align-items: center;
		gap: 0.45rem;
		padding: 0.55rem 0.85rem;
		border-radius: 0.55rem;
		background: color-mix(in srgb, var(--pico-primary) 11%, transparent);
		color: var(--pico-primary);
		font-size: 0.9rem;
		font-weight: 650;
		text-decoration: none;
	}
	nav svg,
	.manage-button svg,
	.repository-card svg {
		width: 1.1rem;
		height: 1.1rem;
		fill: none;
		stroke: currentColor;
		stroke-linecap: round;
		stroke-linejoin: round;
		stroke-width: 1.8;
	}
	.account-mark {
		display: grid;
		width: 2.2rem;
		height: 2.2rem;
		justify-self: end;
		place-items: center;
		border: 1px solid var(--pico-muted-border-color);
		border-radius: 50%;
		background: var(--pico-card-background-color);
	}
	.account-mark svg {
		width: 1.25rem;
		fill: var(--pico-color);
	}
	main {
		width: 100%;
		padding-block: clamp(2.25rem, 5vw, 4.5rem) 5rem;
	}
	.page-heading {
		display: flex;
		align-items: flex-end;
		justify-content: space-between;
		gap: 2rem;
		margin-bottom: 2.5rem;
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
	.page-heading p:last-child {
		margin: 0.75rem 0 0;
		color: var(--pico-muted-color);
	}
	.manage-button {
		display: inline-flex;
		width: auto;
		align-items: center;
		gap: 0.5rem;
		margin: 0;
		white-space: nowrap;
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
	.icon-button {
		width: auto;
		margin: 0;
		padding: 0.15rem 0.45rem;
		border: 0;
		background: transparent;
		color: var(--pico-muted-color);
		font-size: 1.4rem;
	}
	.state-card {
		margin: 0;
		border: 1px solid var(--pico-muted-border-color);
		box-shadow: 0 1.4rem 4rem color-mix(in srgb, var(--pico-color) 5%, transparent);
	}
	.empty-state {
		display: flex;
		min-height: 24rem;
		align-items: center;
		flex-direction: column;
		justify-content: center;
		padding: 3rem 1.5rem;
		text-align: center;
	}
	.empty-illustration {
		display: grid;
		width: 6rem;
		height: 6rem;
		margin-bottom: 1.25rem;
		place-items: center;
		border-radius: 1.5rem;
		background: color-mix(in srgb, var(--pico-primary) 9%, transparent);
		color: var(--pico-primary);
	}
	.empty-illustration svg {
		width: 4.5rem;
		fill: none;
		stroke: currentColor;
		stroke-linecap: round;
		stroke-linejoin: round;
		stroke-width: 2.5;
	}
	.empty-state h2,
	.error-state h2 {
		margin-bottom: 0.5rem;
		font-size: 1.35rem;
	}
	.empty-state p,
	.error-state p {
		max-width: 31rem;
		color: var(--pico-muted-color);
	}
	.empty-state button {
		width: auto;
		margin: 0.5rem 0 0;
	}
	.error-state {
		display: flex;
		align-items: flex-start;
		gap: 1rem;
		padding: 2rem;
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
	.repository-summary {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 1rem;
		margin-bottom: 1rem;
		color: var(--pico-muted-color);
	}
	.repository-summary p {
		margin: 0;
	}
	.repository-summary strong {
		color: var(--pico-color);
	}
	.repository-grid {
		display: grid;
		gap: 1rem;
		grid-template-columns: repeat(2, minmax(0, 1fr));
	}
	.repository-card {
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
	.repository-card:hover {
		border-color: color-mix(in srgb, var(--pico-primary) 45%, var(--pico-muted-border-color));
		transform: translateY(-2px);
	}
	.repository-card header {
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
	.repo-copy {
		flex: 1;
		padding-block: 1.5rem;
	}
	.repo-copy small {
		color: var(--pico-muted-color);
	}
	.repo-copy h2 {
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
		color: var(--pico-muted-color);
		font-size: 0.88rem;
	}
	.repository-card footer {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 1rem;
		padding-top: 1rem;
		border-top: 1px solid var(--pico-muted-border-color);
		font-size: 0.76rem;
	}
	.branch,
	.repository-card footer a {
		display: inline-flex;
		min-width: 0;
		align-items: center;
		gap: 0.35rem;
	}
	.branch {
		overflow: hidden;
		color: var(--pico-muted-color);
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.repository-card footer a {
		flex: 0 0 auto;
		font-weight: 650;
		text-decoration: none;
	}
	.site-footer {
		border-top: 1px solid var(--pico-muted-border-color);
		color: var(--pico-muted-color);
		font-size: 0.72rem;
	}
	.site-footer .container {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding-block: 1.15rem;
	}
	.sync-dot {
		display: inline-block;
		width: 0.45rem;
		height: 0.45rem;
		margin-right: 0.35rem;
		border-radius: 50%;
		background: var(--pico-primary);
	}
	dialog article {
		width: min(42rem, calc(100vw - 2rem));
		margin: 0;
		padding: 0;
		overflow: hidden;
	}
	.dialog-header {
		display: flex;
		align-items: flex-start;
		justify-content: space-between;
		gap: 1.5rem;
		padding: 1.5rem 1.5rem 1rem;
	}
	.dialog-header h2 {
		margin: 0;
		font-size: 1.5rem;
	}
	.dialog-header h2 + p {
		margin: 0.4rem 0 0;
		color: var(--pico-muted-color);
		font-size: 0.9rem;
	}

	.dialog-header small {
		display: block;
		margin-top: 0.25rem;
		color: var(--pico-muted-color);
	}
	dialog form,
	dialog fieldset {
		margin: 0;
	}
	dialog fieldset {
		padding: 0;
		border: 0;
	}
	.repository-options {
		max-height: min(24rem, 48vh);
		overflow-y: auto;
		border-block: 1px solid var(--pico-muted-border-color);
	}
	.repository-option {
		display: flex;
		align-items: flex-start;
		gap: 0.85rem;
		margin: 0;
		padding: 0.9rem 1.5rem;
		cursor: pointer;
	}
	.repository-option + .repository-option {
		border-top: 1px solid var(--pico-muted-border-color);
	}
	.repository-option:hover {
		background: color-mix(in srgb, var(--pico-primary) 5%, transparent);
	}
	.repository-option input {
		margin-top: 0.2rem;
	}
	.repository-option span {
		display: grid;
		min-width: 0;
		gap: 0.2rem;
	}
	.repository-option strong {
		overflow-wrap: anywhere;
	}
	.repository-option small {
		display: -webkit-box;
		overflow: hidden;
		-webkit-box-orient: vertical;
		-webkit-line-clamp: 1;
		line-clamp: 1;
		color: var(--pico-muted-color);
	}
	.dialog-empty {
		padding: 2rem 1.5rem;
		border-block: 1px solid var(--pico-muted-border-color);
		color: var(--pico-muted-color);
		text-align: center;
	}
	.dialog-actions {
		display: flex;
		justify-content: flex-end;
		gap: 0.75rem;
		padding: 1rem 1.5rem 1.5rem;
	}
	.dialog-actions button {
		width: auto;
		margin: 0;
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
	@media (max-width: 700px) {
		.header-inner {
			grid-template-columns: 1fr auto;
		}
		nav {
			display: none;
		}
		.page-heading {
			align-items: stretch;
			flex-direction: column;
			gap: 1.5rem;
		}
		.manage-button {
			width: 100%;
			justify-content: center;
		}
		.repository-summary {
			align-items: flex-start;
			flex-direction: column;
			gap: 0.25rem;
		}
		.repository-grid {
			grid-template-columns: 1fr;
		}
		.repository-card {
			min-height: 15rem;
		}
		.site-footer .container span:last-child {
			display: none;
		}
		dialog article {
			width: calc(100vw - 1rem);
		}
		.dialog-actions button {
			flex: 1;
		}
	}
</style>
