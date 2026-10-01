<script lang="ts">
	import { onNavigate } from '$app/navigation';
	import favicon from '$lib/assets/favicon.svg';
	import '../app.css';

	let { children } = $props();

	onNavigate((navigation) => {
		if (!document.startViewTransition || matchMedia('(prefers-reduced-motion: reduce)').matches) return;
		return new Promise((resolve) => {
			document.startViewTransition(async () => {
				resolve();
				await navigation.complete;
			});
		});
	});
</script>

<!-- iOS Safari applies :active only on a page that listens for touches. -->
<svelte:body ontouchstart={() => {}} />

<svelte:head>
	<link rel="icon" href={favicon} />
</svelte:head>

<div class="mx-auto min-h-dvh w-full max-w-[430px] bg-ground text-ink">
	{@render children()}
</div>
