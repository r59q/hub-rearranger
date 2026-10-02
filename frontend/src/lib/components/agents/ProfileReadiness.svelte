<script lang="ts">
	import type { AgentProfile } from '$lib/agents/types';
	import {
		evidenceReason,
		formatTimestamp,
		runtimeLabel,
		setupLabel
	} from '$lib/agents/presentation';

	let { profile, stale = false }: { profile: AgentProfile; stale?: boolean } = $props();

	let evidence = $derived(profile.readiness?.evidence);
	let verified = $derived(!stale && profile.readiness?.state === 'runtime_verified');
	let failed = $derived(!stale && profile.readiness?.state === 'verification_failed');
</script>

<section class="readiness" aria-labelledby={'readiness-' + profile.id}>
	<h3 id={'readiness-' + profile.id}>Configuration &amp; runtime</h3>
	<dl>
		<div>
			<dt>Profile catalog</dt>
			<dd>Valid at this revision</dd>
		</div>
		<div>
			<dt>Repository setup</dt>
			<dd>{setupLabel(profile, stale)}</dd>
		</div>
		<div>
			<dt>Runtime</dt>
			<dd class:verified class:failed>{runtimeLabel(profile, stale)}</dd>
		</div>
	</dl>
	<div class="next-action">
		<strong>Next step</strong>
		<p>
			{stale
				? 'Refresh profiles to check the current configuration and runtime evidence.'
				: (profile.readiness?.nextAction ?? 'Refresh profiles when readiness is available again.')}
		</p>
	</div>
	{#if !stale && profile.readiness && profile.readiness.diagnostics.length > 0}
		<details open={profile.readiness.state === 'configuration_missing'}>
			<summary>Setup diagnostics ({profile.readiness.diagnostics.length})</summary>
			<ul>
				{#each profile.readiness.diagnostics as diagnostic, index (index)}
					<li>
						<code>{diagnostic.path}</code>
						<p>{diagnostic.message}</p>
					</li>
				{/each}
			</ul>
		</details>
	{/if}
	{#if evidence}
		<details>
			<summary>{stale ? 'Previous diagnostic evidence' : 'Diagnostic evidence'}</summary>
			<p>{evidenceReason(evidence.reason)}</p>
			<dl>
				<div>
					<dt>Checked</dt>
					<dd>
						<time datetime={evidence.verifiedAt}>{formatTimestamp(evidence.verifiedAt)}</time>
					</dd>
				</div>
				<div>
					<dt>Requested policy</dt>
					<dd>{evidence.requestedModel} · {evidence.requestedEffort} reasoning</dd>
				</div>
				<div>
					<dt>Effective policy</dt>
					<dd>
						{evidence.effectiveModel && evidence.effectiveEffort
							? `${evidence.effectiveModel} · ${evidence.effectiveEffort} reasoning`
							: 'Not verified'}
					</dd>
				</div>
				<div>
					<dt>Codex version</dt>
					<dd>{evidence.cliVersion ?? 'Not verified'}</dd>
				</div>
			</dl>
			<!-- Evidence links are built from validated repository/run identifiers on the server. -->
			<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
			<a href={evidence.runUrl} target="_blank" rel="noreferrer">Diagnostic run on GitHub ↗</a>
		</details>
	{/if}
</section>

<style>
	.readiness {
		min-width: 0;
	}
	h3 {
		margin: 0 0 1rem;
		font-size: 1rem;
	}
	dl {
		margin: 0;
		font-size: 0.85rem;
	}
	dl > div {
		display: grid;
		grid-template-columns: minmax(8rem, 0.8fr) minmax(0, 1.2fr);
		gap: 0.5rem 1rem;
		margin-bottom: 0.75rem;
	}
	dt {
		color: var(--pico-muted-color);
	}
	dd {
		margin: 0;
		overflow-wrap: anywhere;
	}
	.verified {
		color: var(--pico-primary);
		font-weight: 750;
	}
	.failed {
		color: var(--pico-del-color);
		font-weight: 750;
	}
	.next-action {
		margin-top: 1.25rem;
		padding: 1rem;
		border-left: 3px solid var(--pico-primary);
		border-radius: 0 0.35rem 0.35rem 0;
		background: color-mix(in srgb, var(--pico-primary) 6%, var(--pico-card-background-color));
		font-size: 0.85rem;
	}
	.next-action p {
		margin: 0.35rem 0 0;
	}
	details {
		margin: 1rem 0 0;
		font-size: 0.85rem;
	}
	summary {
		min-height: 2.75rem;
		padding-block: 0.75rem;
	}
	details > p {
		margin-top: 0.5rem;
	}
	ul {
		padding-left: 1.2rem;
	}
	li {
		margin-block: 0.75rem;
		overflow-wrap: anywhere;
	}
	li p {
		margin: 0.4rem 0 0;
	}
	code {
		overflow-wrap: anywhere;
		white-space: normal;
	}
	@media (max-width: 480px) {
		dl > div {
			grid-template-columns: 1fr;
			gap: 0.15rem;
		}
	}
</style>
