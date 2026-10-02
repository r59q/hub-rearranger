<script lang="ts">
	import type { AgentProfile } from '$lib/agents/types';
	import { readinessLabels } from '$lib/agents/presentation';
	import ProfileReadiness from './ProfileReadiness.svelte';

	let { profile, stale = false }: { profile: AgentProfile; stale?: boolean } = $props();
</script>

<article aria-labelledby={'profile-' + profile.id}>
	<header>
		<div>
			<p class="profile-id">
				<code>{profile.id}</code> · {profile.enabled ? 'Enabled' : 'Disabled'}
			</p>
			<h2 id={'profile-' + profile.id}>{profile.name}</h2>
			<p class="description">{profile.description}</p>
		</div>
		<span
			class="state-badge"
			class:verified={!stale && profile.readiness?.state === 'runtime_verified'}
			class:failed={!stale && profile.readiness?.state === 'verification_failed'}
		>
			{stale
				? 'Results need refreshing'
				: profile.readiness
					? readinessLabels[profile.readiness.state]
					: 'Readiness unavailable'}
		</span>
	</header>
	{#if !profile.enabled}<p class="disabled-note">
			This profile is disabled in the repository catalog.
		</p>{/if}
	<div class="profile-body">
		<section aria-labelledby={'policy-' + profile.id}>
			<h3 id={'policy-' + profile.id}>Profile policy</h3>
			<dl>
				<div>
					<dt>Role</dt>
					<dd>{profile.role === 'implementation' ? 'Implementation' : profile.role}</dd>
				</div>
				<div>
					<dt>Authority</dt>
					<dd>
						{profile.authority === 'branch-draft-pr'
							? 'Branch and draft pull request'
							: profile.authority}
					</dd>
				</div>
				<div>
					<dt>Model</dt>
					<dd><code>{profile.model}</code> · {profile.effort} reasoning</dd>
				</div>
				<div>
					<dt>Runner label</dt>
					<dd><code>{profile.runner}</code></dd>
				</div>
			</dl>
			<p class="runner-note">Requires a dedicated Linux self-hosted runner with this label.</p>
			<details>
				<summary>Execution and review policy</summary>
				<dl>
					<div>
						<dt>Workspace</dt>
						<dd>
							{profile.sandbox === 'workspace-write'
								? 'Changes allowed in the workspace'
								: profile.sandbox}
						</dd>
					</div>
					<div>
						<dt>Workload network</dt>
						<dd>{profile.network ? 'Allowed' : 'Blocked'}</dd>
					</div>
					<div>
						<dt>Validation</dt>
						<dd>{profile.checks.join(', ')}</dd>
					</div>
					<div>
						<dt>Review follow-up</dt>
						<dd>{profile.reviewComments ? 'Enabled for trusted review comments' : 'Disabled'}</dd>
					</div>
				</dl>
			</details>
		</section>
		<ProfileReadiness {profile} {stale} />
	</div>
</article>

<style>
	article {
		min-width: 0;
		margin: 0;
		padding: clamp(1rem, 3vw, 1.75rem);
		border: 1px solid var(--pico-muted-border-color);
		box-shadow: 0 0.6rem 2rem color-mix(in srgb, var(--pico-color) 4%, transparent);
	}
	header {
		display: flex;
		align-items: flex-start;
		justify-content: space-between;
		gap: 1.25rem;
		margin: 0 0 1.5rem;
		padding: 0 0 1.5rem;
		background: transparent;
		border-bottom: 1px solid var(--pico-muted-border-color);
	}
	header > div {
		min-width: 0;
	}
	.profile-id {
		margin: 0 0 0.5rem;
		color: var(--pico-muted-color);
		font-size: 0.75rem;
	}
	h2 {
		margin: 0;
		font-size: 1.45rem;
		letter-spacing: -0.02em;
		overflow-wrap: anywhere;
	}
	.description {
		margin: 0.5rem 0 0;
		color: var(--pico-muted-color);
		font-size: 0.85rem;
		overflow-wrap: anywhere;
	}
	.state-badge {
		max-width: 100%;
		flex: 0 0 auto;
		padding: 0.4rem 0.75rem;
		border: 1px solid var(--pico-muted-border-color);
		border-radius: 2rem;
		font-size: 0.75rem;
		font-weight: 650;
		color: var(--pico-color);
	}
	.state-badge.verified {
		border-color: var(--pico-primary);
		color: var(--pico-primary);
	}
	.state-badge.failed {
		border-color: var(--pico-del-color);
		color: var(--pico-del-color);
	}
	.profile-body {
		display: grid;
		grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
		gap: 2.5rem;
	}
	section {
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
		grid-template-columns: minmax(6rem, 0.7fr) minmax(0, 1.3fr);
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
	code {
		white-space: normal;
		overflow-wrap: anywhere;
	}
	.runner-note,
	.disabled-note {
		color: var(--pico-muted-color);
		font-size: 0.8rem;
	}
	.runner-note {
		margin: 1rem 0 0;
	}
	details {
		margin: 1rem 0 0;
		font-size: 0.85rem;
	}
	summary {
		min-height: 2.75rem;
		padding-block: 0.75rem;
	}
	@media (max-width: 900px) {
		.profile-body {
			grid-template-columns: 1fr;
			gap: 1.5rem;
		}
		header {
			flex-direction: column;
		}
	}
	@media (max-width: 480px) {
		dl > div {
			grid-template-columns: 1fr;
			gap: 0.15rem;
		}
	}
</style>
