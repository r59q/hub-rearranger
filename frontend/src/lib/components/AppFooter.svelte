<footer>
	<div class="container">
		<span><span class="sync-dot" aria-hidden="true"></span>GitHub is the source of truth</span>
		<span class="product-name">Hub Rearranger</span>
	</div>
</footer>

<style>
	footer {
		border-top: 1px solid var(--pico-muted-border-color);
		color: var(--pico-muted-color);
		font-size: 0.72rem;
	}

	.container {
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

	@media (max-width: 700px) {
		.product-name {
			display: none;
		}
	}
</style>
