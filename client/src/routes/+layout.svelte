<script lang="ts">
	import { beforeNavigate, onNavigate } from '$app/navigation';
	import { navigating } from '$app/state';
	import favicon from '$lib/assets/favicon.svg';
	import { tabOf, tabs } from '$lib/tabs';
	import '../app.css';

	let { children } = $props();

	// How a page arrives: one deeper slides in from the right, one higher
	// slides back, another tab fades in, and a change within a page just
	// shows.
	type Kind = 'forward' | 'back' | 'tab' | 'none';
	let kind = $state<Kind>('none');

	const isRoot = (u: URL) => tabs.some((t) => t.roots.includes(u.pathname));
	const depth = (u: URL) => u.pathname.split('/').filter(Boolean).length;

	function kindOf(from: URL, to: URL, type: string): Kind {
		if (type === 'popstate') return swiped ? 'none' : 'back';
		if (from.pathname === to.pathname) {
			const a = Number(from.searchParams.get('step')), b = Number(to.searchParams.get('step'));
			return a && b && a !== b ? (b > a ? 'forward' : 'back') : 'none';
		}
		if (isRoot(to) && tabOf(from.pathname) !== tabOf(to.pathname)) return 'tab';
		if (isRoot(from) && isRoot(to)) return 'none';
		return depth(to) >= depth(from) ? 'forward' : 'back';
	}

	// Safari in a browser tab draws its own slide for a swipe back.
	let swiped = false;
	$effect(() => {
		const note = (e: PopStateEvent) => (swiped = 'hasUAVisualTransition' in e && !!e.hasUAVisualTransition);
		addEventListener('popstate', note, true);
		return () => removeEventListener('popstate', note, true);
	});

	beforeNavigate(({ from, to, type }) => {
		if (from && to) kind = kindOf(from.url, to.url, type);
	});

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
		if (kind === 'none') return;
		document.documentElement.dataset.nav = kind;
		return new Promise((resolve) => {
			const t = document.startViewTransition(async () => {
				resolve();
				await navigation.complete;
			});
			t.finished.finally(() => delete document.documentElement.dataset.nav);
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
