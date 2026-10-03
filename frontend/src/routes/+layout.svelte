<script lang="ts">
	import type { Snippet } from 'svelte';
	import AppFooter from '$lib/components/AppFooter.svelte';
	import AppHeader from '$lib/components/AppHeader.svelte';
	import WorkdeskTabs from '$lib/components/WorkdeskTabs.svelte';
	import '../app.css';

	let { children }: { children: Snippet } = $props();
</script>

<div class="site-shell">
	<AppHeader />
	<WorkdeskTabs />
	{@render children()}
	<AppFooter />
</div>

<style>
	.site-shell {
		display: grid;
		min-height: 100vh;
		grid-template-rows: auto auto 1fr auto;
		grid-template-columns: minmax(0, 1fr);
		background:
			radial-gradient(
				circle at 82% 0%,
				color-mix(in srgb, var(--pico-primary) 7%, transparent),
				transparent 24rem
			),
			var(--pico-background-color);
	}
</style>
