<script lang="ts">
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	let connecting = $state(false);
	let disconnecting = $state(false);

	const notices: Record<string, string> = {
		'signed-out': 'Signed out. Your GitHub user token was revoked.',
		'revocation-unconfirmed':
			'Signed out of Hub. GitHub token revocation could not be confirmed. Revoke the App authorization in GitHub account settings to finish disconnecting.',
		reconnect: 'Your GitHub connection expired or was revoked. Sign in again.',
		'request-rejected': 'The sign-in request could not be verified. Start again from this page.',
		unavailable: 'GitHub sign-in is unavailable. Try again.'
	};

	let notice = $derived(notices[page.url.searchParams.get('notice') ?? '']);
</script>

<svelte:head><title>GitHub account · Hub Rearranger</title></svelte:head>
<main class="container">
	<header>
		<p class="eyebrow">Account</p>
		<h1>GitHub connection</h1>
		<p>Connect your GitHub identity for repository actions in Hub.</p>
	</header>
	{#if notice}<p class="notice" role="status">{notice}</p>{/if}
	<section aria-labelledby="connection-title">
		{#if data.identity.state === 'authenticated' && data.identity.user}
			<h2 id="connection-title">Signed in as {data.identity.user.login}</h2>
			<p>
				Repository actions use your GitHub identity. Hub checks your current access and the App’s
				permissions for each action.
			</p>
			<p>
				Your Hub session ends <time datetime={data.identity.expiresAt ?? undefined}
					>{new Date(data.identity.expiresAt!).toISOString().slice(0, 16).replace('T', ' ')} UTC</time
				>. You may be asked to reconnect sooner if GitHub access changes.
			</p>
			<form
				method="POST"
				action={resolve('/auth/sign-out')}
				onsubmit={() => {
					disconnecting = true;
				}}
			>
				<input type="hidden" name="csrf" value={data.identity.csrf ?? ''} /><button
					class="outline"
					aria-busy={disconnecting}
					disabled={disconnecting}
					>{disconnecting ? 'Signing out…' : 'Sign out and revoke token'}</button
				>
			</form>
			<p class="muted">
				Signing out removes this Hub session and asks GitHub to revoke its user token.
			</p>
		{:else if data.identity.state === 'disabled'}
			<h2 id="connection-title">GitHub sign-in is not configured</h2>
			<p>
				The Hub operator needs to configure a GitHub App before you can connect your account.
				Repository browsing remains available.
			</p>
		{:else if data.identity.state === 'unavailable'}
			<h2 id="connection-title">GitHub connection is unavailable</h2>
			<p>
				Your account status could not be checked. Try again when the identity service is available.
			</p>
			<a href={resolve('/account')} data-sveltekit-reload>Check connection again</a>
		{:else}
			<h2 id="connection-title">
				{data.identity.state === 'reconnect_required'
					? 'Reconnect your GitHub account'
					: 'Sign in with GitHub'}
			</h2>
			<p>
				Continue to GitHub to authorize Hub’s App. Connecting your account does not change
				repository files or submit an agent assignment.
			</p>
			<form
				method="POST"
				action={resolve('/auth/start')}
				onsubmit={() => {
					connecting = true;
				}}
			>
				<button aria-busy={connecting} disabled={connecting}
					>{connecting ? 'Continuing…' : 'Continue to GitHub'}</button
				>
			</form>
		{/if}
		<details>
			<summary>Repository access and permissions</summary>
			<p>
				Assignment comments require your current maintainer or admin role and the App’s Issues write
				permission. Bootstrap pull requests require repository write access plus the App’s Contents,
				Pull requests, and Workflows write permissions.
			</p>
			<p>
				The App can act only on repositories available to both you and its installation. GitHub
				credentials stay on the server.
			</p>
			<!-- This fixed external link opens GitHub account authorization settings. -->
			<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
			<a href="https://github.com/settings/apps" target="_blank" rel="noreferrer"
				>Manage authorized GitHub Apps ↗</a
			>
		</details>
	</section>
	<a href={resolve('/')}>Back to workspace</a>
</main>

<style>
	main {
		padding-block: 3rem 5rem;
		max-width: 48rem;
	}
	header {
		margin-bottom: 2rem;
	}
	h1 {
		font-size: clamp(2rem, 5vw, 3rem);
		letter-spacing: -0.04em;
	}
	h2 {
		font-size: 1.3rem;
		overflow-wrap: anywhere;
	}
	.eyebrow {
		color: var(--pico-primary);
		font-size: 0.8rem;
		text-transform: uppercase;
		letter-spacing: 0.1em;
	}
	section,
	.notice {
		padding: clamp(1rem, 3vw, 1.75rem);
		border: 1px solid var(--pico-muted-border-color);
		border-radius: var(--pico-border-radius);
		background: var(--pico-card-background-color);
		margin-bottom: 1.5rem;
	}
	.notice {
		border-left: 3px solid var(--pico-primary);
	}
	p {
		font-size: 0.9rem;
		overflow-wrap: anywhere;
	}
	.muted {
		color: var(--pico-muted-color);
		font-size: 0.8rem;
	}
	form {
		margin-bottom: 0.75rem;
	}
	button {
		margin: 0;
		min-height: 2.75rem;
	}
	details {
		margin-top: 1.5rem;
	}
	summary {
		min-height: 2.75rem;
		padding-block: 0.75rem;
	}
	@media (max-width: 480px) {
		button {
			width: 100%;
		}
	}
</style>
