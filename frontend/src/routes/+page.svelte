<svelte:head>
	<title>Hub Rearranger</title>
	<meta
		name="description"
		content="A focused workspace for GitHub workflows shared by people and AI agents."
	/>
</svelte:head>

<main>
	<section aria-labelledby="page-title">
		<p class="eyebrow">Hub Rearranger</p>
		<h1 id="page-title">A clearer workspace for GitHub work.</h1>
		<p class="intro">
			Connect a repository to bring its active work, review state, and next actions into one focused
			place.
		</p>
		<p class="status" role="status">Repository connections will be available here soon.</p>
	</section>
</main>

<style>
	:global(*) {
		box-sizing: border-box;
	}

	:global(body) {
		margin: 0;
		background: #f6f8fa;
		color: #1f2328;
		font-family:
			Inter,
			ui-sans-serif,
			system-ui,
			-apple-system,
			BlinkMacSystemFont,
			'Segoe UI',
			sans-serif;
	}

	main {
		display: grid;
		min-height: 100vh;
		place-items: center;
		padding: 2rem;
	}

	section {
		width: min(100%, 42rem);
		border: 1px solid #d0d7de;
		border-radius: 0.75rem;
		background: #ffffff;
		padding: clamp(2rem, 7vw, 4.5rem);
		box-shadow: 0 1px 3px rgb(31 35 40 / 12%);
	}

	.eyebrow {
		margin: 0 0 1rem;
		color: #0969da;
		font-size: 0.875rem;
		font-weight: 700;
		letter-spacing: 0.08em;
		text-transform: uppercase;
	}

	h1 {
		margin: 0;
		font-size: clamp(2rem, 6vw, 3.5rem);
		line-height: 1.05;
	}

	.intro {
		margin: 1.5rem 0 0;
		color: #57606a;
		font-size: 1.125rem;
		line-height: 1.6;
	}

	.status {
		margin: 2rem 0 0;
		border-left: 3px solid #1f883d;
		padding-left: 0.75rem;
		color: #1a7f37;
		font-size: 0.9375rem;
		font-weight: 600;
	}
</style>
