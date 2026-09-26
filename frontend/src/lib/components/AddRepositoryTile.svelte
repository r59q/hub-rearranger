<script lang="ts">
	import { enhance as enhanceForm } from '$app/forms';
	import type { SubmitFunction } from '@sveltejs/kit';
	import type { Repository } from '$lib/repositories/types';

	let {
		repository,
		submit,
		busy,
		disabled
	}: {
		repository: Repository;
		submit: SubmitFunction;
		busy: boolean;
		disabled: boolean;
	} = $props();
</script>

<article>
	<div class="repo-icon" aria-hidden="true">
		<svg viewBox="0 0 24 24"
			><path
				d="M5 3.75h11.25A2.75 2.75 0 0 1 19 6.5v12.75H7.75A2.75 2.75 0 0 1 5 16.5V3.75Zm0 12.75a2.75 2.75 0 0 1 2.75-2.75H19M8.5 7.5h7"
			/></svg
		>
	</div>
	<div class="repo-copy">
		<div class="repo-title">
			<h3>{repository.name}</h3>
			<span>{repository.private ? 'Private' : 'Public'}</span>
		</div>
		<small>{repository.owner}</small>
		<p>{repository.description || 'No description provided on GitHub.'}</p>
	</div>
	<form method="POST" action="?/addRepository" use:enhanceForm={submit}>
		<input type="hidden" name="repositoryId" value={repository.id} />
		<button
			type="submit"
			aria-label={`Add ${repository.full_name} to workspace`}
			aria-busy={busy}
			{disabled}
		>
			{#if !busy}
				<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 5v14M5 12h14" /></svg>
			{/if}
		</button>
	</form>
</article>

<style>
	article {
		display: grid;
		align-items: center;
		gap: 1rem;
		grid-template-columns: auto minmax(0, 1fr) auto;
		margin: 0;
		padding: 1rem;
		border: 1px solid var(--pico-muted-border-color);
		box-shadow: 0 0.4rem 1.2rem color-mix(in srgb, var(--pico-color) 3%, transparent);
		transition:
			border-color 160ms ease,
			transform 160ms ease;
	}

	article:hover,
	article:focus-within {
		border-color: color-mix(in srgb, var(--pico-primary) 45%, var(--pico-muted-border-color));
		transform: translateY(-1px);
	}

	.repo-icon {
		display: grid;
		width: 2.5rem;
		height: 2.5rem;
		place-items: center;
		border-radius: 0.65rem;
		background: color-mix(in srgb, var(--pico-primary) 9%, transparent);
		color: var(--pico-primary);
	}

	.repo-icon svg,
	button svg {
		width: 1.15rem;
		height: 1.15rem;
		fill: none;
		stroke: currentColor;
		stroke-linecap: round;
		stroke-linejoin: round;
		stroke-width: 1.8;
	}

	.repo-copy {
		min-width: 0;
	}

	.repo-title {
		display: flex;
		min-width: 0;
		align-items: center;
		gap: 0.5rem;
	}

	h3 {
		overflow: hidden;
		margin: 0;
		font-size: 1rem;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.repo-title span {
		padding: 0.1rem 0.4rem;
		border: 1px solid var(--pico-muted-border-color);
		border-radius: 2rem;
		color: var(--pico-muted-color);
		font-size: 0.6rem;
	}

	.repo-copy small,
	.repo-copy p {
		color: var(--pico-muted-color);
	}

	.repo-copy small {
		display: block;
		margin-top: 0.1rem;
		font-size: 0.68rem;
	}

	.repo-copy p {
		overflow: hidden;
		margin: 0.45rem 0 0;
		font-size: 0.76rem;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	form {
		margin: 0;
	}

	button {
		display: grid;
		width: 2.35rem;
		height: 2.35rem;
		margin: 0;
		padding: 0;
		place-items: center;
		border-radius: 50%;
	}

	@media (max-width: 420px) {
		article {
			gap: 0.75rem;
		}

		.repo-icon {
			display: none;
		}
	}
</style>
