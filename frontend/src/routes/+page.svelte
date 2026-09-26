<script lang="ts">
	import { resolve } from '$app/paths';
</script>

<svelte:head>
	<title>Hub Rearranger</title>
	<meta
		name="description"
		content="A focused workspace for GitHub workflows shared by people and AI agents."
	/>
</svelte:head>

<div class="site-shell">
	<header class="container">
		<nav aria-label="Primary navigation">
			<ul>
				<li>
					<a class="brand" href={resolve('/')} aria-label="Hub Rearranger home">
						<span class="brand-mark" aria-hidden="true">H</span>
						<strong>Hub Rearranger</strong>
					</a>
				</li>
			</ul>
			<ul class="product-context">
				<li><small>Agent-ready GitHub workspace</small></li>
			</ul>
		</nav>
	</header>

	<main class="container">
		<section class="hero" aria-labelledby="page-title">
			<div class="hero-copy">
				<p class="eyebrow">GitHub, arranged around the work</p>
				<h1 id="page-title">A clearer workspace for GitHub workflows.</h1>
				<p class="intro">
					Bring active work, review state, ownership, and next actions together in one focused place
					for people and their agents.
				</p>
			</div>

			<article class="workspace-status" aria-labelledby="workspace-status-title">
				<header>
					<div>
						<small>Workspace status</small>
						<h2 id="workspace-status-title">Ready to connect</h2>
					</div>
					<span class="status-dot" aria-hidden="true"></span>
				</header>
				<p>Repository connections will appear here as the GitHub integration becomes available.</p>
				<footer>
					<small><strong>Next:</strong> connect a repository and choose work to focus on.</small>
				</footer>
			</article>
		</section>
	</main>

	<footer class="container site-footer">
		<small>GitHub remains the source of truth.</small>
	</footer>
</div>

<style>
	.site-shell {
		display: grid;
		min-height: 100vh;
		grid-template-rows: auto 1fr auto;
		background:
			radial-gradient(
				circle at 85% 15%,
				color-mix(in srgb, var(--pico-primary) 12%, transparent),
				transparent 28rem
			),
			var(--pico-background-color);
	}

	nav {
		padding-block: 1rem;
	}

	.brand {
		display: inline-flex;
		align-items: center;
		gap: 0.7rem;
		color: var(--pico-color);
		text-decoration: none;
	}

	.brand-mark {
		display: grid;
		width: 2rem;
		height: 2rem;
		place-items: center;
		border-radius: 0.6rem;
		background: var(--pico-primary-background);
		color: var(--pico-primary-inverse);
		font-weight: 800;
		box-shadow: 0 0.5rem 1.5rem color-mix(in srgb, var(--pico-primary) 22%, transparent);
	}

	main {
		display: grid;
		align-items: center;
		padding-block: clamp(3rem, 10vw, 8rem);
	}

	.hero {
		display: grid;
		align-items: center;
		gap: clamp(2.5rem, 8vw, 7rem);
		grid-template-columns: minmax(0, 1.15fr) minmax(18rem, 0.85fr);
	}

	.eyebrow {
		margin-bottom: 1rem;
		color: var(--pico-primary);
		font-size: 0.875rem;
		font-weight: 700;
		letter-spacing: 0.1em;
		text-transform: uppercase;
	}

	h1 {
		max-width: 14ch;
		margin-bottom: 1.5rem;
		font-size: clamp(2.5rem, 7vw, 5rem);
		letter-spacing: -0.045em;
		line-height: 1;
	}

	.intro {
		max-width: 56ch;
		margin: 0;
		color: var(--pico-muted-color);
		font-size: clamp(1.05rem, 2vw, 1.25rem);
		line-height: 1.6;
	}

	.workspace-status {
		margin: 0;
		border: 1px solid var(--pico-muted-border-color);
		box-shadow: 0 1.5rem 4rem color-mix(in srgb, var(--pico-color) 9%, transparent);
	}

	.workspace-status header {
		display: flex;
		align-items: flex-start;
		justify-content: space-between;
		gap: 1rem;
	}

	.workspace-status h2 {
		margin: 0.25rem 0 0;
		font-size: 1.35rem;
	}

	.workspace-status p {
		color: var(--pico-muted-color);
	}

	.status-dot {
		width: 0.75rem;
		height: 0.75rem;
		margin-top: 0.45rem;
		flex: 0 0 auto;
		border-radius: 50%;
		background: var(--pico-primary);
		box-shadow: 0 0 0 0.3rem color-mix(in srgb, var(--pico-primary) 18%, transparent);
	}

	.site-footer {
		padding-block: 1.5rem;
		color: var(--pico-muted-color);
	}

	@media (max-width: 700px) {
		.product-context {
			display: none;
		}

		main {
			padding-block: 3rem;
		}

		.hero {
			gap: 2.5rem;
			grid-template-columns: 1fr;
		}

		h1 {
			font-size: clamp(2.5rem, 14vw, 4rem);
		}
	}
</style>
