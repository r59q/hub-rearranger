<script lang="ts">
	import { resolve } from '$app/paths';
	import { navigating, page } from '$app/state';

	const dashboardHref = resolve('/');
	const issuesHref = resolve('/issues');
	const agentsHref = resolve('/agents');

	let loadingAgents = $derived(navigating.to?.url.pathname === agentsHref);
</script>

<div class="workdesk-bar">
	<div class="container workdesk-inner">
		<span class="workdesk-label">Workdesk</span>
		<nav aria-label="Workdesk pages">
			<a
				href={dashboardHref}
				aria-current={page.url.pathname === dashboardHref ? 'page' : undefined}
			>
				<svg viewBox="0 0 24 24" aria-hidden="true">
					<path d="M4 4h6v7H4V4Zm10 0h6v4h-6V4ZM4 15h6v5H4v-5Zm10-3h6v8h-6v-8Z" />
				</svg>
				Dashboard
			</a>
			<a href={issuesHref} aria-current={page.url.pathname === issuesHref ? 'page' : undefined}>
				<svg viewBox="0 0 24 24" aria-hidden="true">
					<path d="M9 4 7 20m10-16-2 16M4 9h16M3 15h16" />
				</svg>
				Issues
			</a>
			<a
				href={agentsHref}
				aria-current={page.url.pathname === agentsHref ? 'page' : undefined}
				aria-busy={loadingAgents}
			>
				<svg viewBox="0 0 24 24" aria-hidden="true"
					><path
						d="M12 3v3m-7 5H3m18 0h-2M7 6h10a2 2 0 0 1 2 2v10H5V8a2 2 0 0 1 2-2ZM9 11h.01M15 11h.01M9 15h6"
					/></svg
				>
				{loadingAgents ? 'Loading agents…' : 'Agents'}
			</a>
		</nav>
	</div>
</div>

<style>
	.workdesk-bar {
		border-bottom: 1px solid var(--pico-muted-border-color);
		background: color-mix(in srgb, var(--pico-card-background-color) 70%, transparent);
	}

	.workdesk-inner {
		display: flex;
		min-width: 0;
		align-items: center;
		gap: 1.5rem;
	}

	.workdesk-label {
		flex: 0 0 auto;
		color: var(--pico-muted-color);
		font-size: 0.68rem;
		font-weight: 750;
		letter-spacing: 0.12em;
		text-transform: uppercase;
	}

	nav {
		display: flex;
		min-width: 0;
		gap: 0.3rem;
		overflow-x: auto;
	}

	a {
		display: inline-flex;
		align-items: center;
		gap: 0.4rem;
		padding: 0.85rem 0.75rem 0.7rem;
		border-bottom: 2px solid transparent;
		color: var(--pico-muted-color);
		font-size: 0.8rem;
		font-weight: 650;
		text-decoration: none;
		white-space: nowrap;
	}

	a:hover {
		color: var(--pico-color);
	}

	a[aria-current='page'] {
		border-bottom-color: var(--pico-primary);
		color: var(--pico-primary);
	}

	svg {
		width: 1rem;
		height: 1rem;
		fill: none;
		stroke: currentColor;
		stroke-linecap: round;
		stroke-linejoin: round;
		stroke-width: 1.8;
	}

	@media (max-width: 700px) {
		.workdesk-inner {
			align-items: flex-start;
			flex-direction: column;
			gap: 0;
			padding-top: 0.7rem;
		}

		nav {
			width: 100%;
		}
	}
</style>
