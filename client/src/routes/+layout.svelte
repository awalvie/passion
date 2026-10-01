<script lang="ts">
	import { onNavigate } from '$app/navigation';
	import { navigating } from '$app/state';
	import favicon from '$lib/assets/favicon.svg';
	import '../app.css';

	let { children } = $props();

	// A page that takes a moment to load shows a bar; a quick one shows nothing.
	let slow = $state(false);
	$effect(() => {
		if (!navigating.to) {
			slow = false;
			return;
		}
		const t = setTimeout(() => (slow = true), 150);
		return () => clearTimeout(t);
	});

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

{#if slow}
	<div class="fixed inset-x-0 top-[env(safe-area-inset-top)] z-[60] h-[3px] overflow-hidden" role="progressbar" aria-label="Loading">
		<div class="h-full w-1/3 bg-ink motion-safe:animate-[load_1s_ease-in-out_infinite] motion-reduce:w-full"></div>
	</div>
{/if}

<div class="mx-auto min-h-dvh w-full max-w-[430px] bg-ground text-ink">
	{@render children()}
</div>
