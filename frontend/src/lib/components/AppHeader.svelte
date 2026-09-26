<script lang="ts">
	import { resolve } from '$app/paths';
</script>

<header class="app-header">
	<div class="container header-inner">
		<a class="brand" href={resolve('/')} aria-label="Hub Rearranger home">
			<span class="brand-mark" aria-hidden="true">H</span><span>Hub Rearranger</span>
		</a>
		<nav aria-label="Primary navigation">
			<a href={resolve('/')} aria-current="page">
				<svg viewBox="0 0 24 24" aria-hidden="true">
					<path
						d="M4 5.5A2.5 2.5 0 0 1 6.5 3h11A2.5 2.5 0 0 1 20 5.5v13a2.5 2.5 0 0 1-2.5 2.5h-11A2.5 2.5 0 0 1 4 18.5v-13ZM8 8h8m-8 4h8m-8 4h5"
					/>
				</svg>
				Repositories
			</a>
		</nav>
		<div class="account-mark" aria-label="GitHub account configured">
			<svg viewBox="0 0 24 24" aria-hidden="true">
				<path
					d="M12 2a10 10 0 0 0-3.16 19.49c.5.1.68-.22.68-.48v-1.86c-2.78.6-3.37-1.18-3.37-1.18-.45-1.16-1.11-1.47-1.11-1.47-.91-.62.07-.61.07-.61 1 .07 1.53 1.03 1.53 1.03.9 1.53 2.35 1.09 2.92.83.09-.65.35-1.09.64-1.34-2.22-.25-4.56-1.11-4.56-4.94 0-1.09.39-1.98 1.03-2.68-.1-.25-.45-1.27.1-2.64 0 0 .84-.27 2.75 1.02A9.56 9.56 0 0 1 12 6.83a9.5 9.5 0 0 1 2.5.34c1.91-1.29 2.75-1.02 2.75-1.02.55 1.37.2 2.39.1 2.64.64.7 1.03 1.59 1.03 2.68 0 3.84-2.34 4.68-4.57 4.93.36.31.68.92.68 1.85v2.76c0 .27.18.58.69.48A10 10 0 0 0 12 2Z"
				/>
			</svg>
		</div>
	</div>
</header>

<style>
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

	nav svg {
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

	@media (max-width: 700px) {
		.header-inner {
			grid-template-columns: 1fr auto;
		}

		nav {
			display: none;
		}
	}
</style>
