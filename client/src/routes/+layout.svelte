<script lang="ts">
	import { beforeNavigate, onNavigate } from '$app/navigation';
	import { navigating, page } from '$app/state';
	import EdgeBack from '$lib/EdgeBack.svelte';
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

	// From lg the Library's list stays put and an item only swaps the pane
	// beside it, so it neither slides nor shows an outline.
	const paned = (id: string | null | undefined) => !!id?.startsWith('/(app)/(tabs)/(library)/');
	let inPane = false;

	beforeNavigate(({ from, to, type }) => {
		if (!from || !to) return;
		inPane = paned(from.route.id) && paned(to.route.id) && matchMedia('(min-width: 64rem)').matches;
		kind = inPane ? 'none' : kindOf(from.url, to.url, type);
	});

	// A page that takes a moment to load shows its outline at once, and its
	// content replaces the outline when it comes.
	let outline = $state(false);
	$effect(() => {
		if (!navigating.to || inPane) {
			outline = false;
			return;
		}
		const t = setTimeout(() => (outline = true), 100);
		return () => clearTimeout(t);
	});
	const outlineRoot = $derived(navigating.to?.url ? isRoot(navigating.to.url) : false);

	// Today, Plan, a cycle, Journal, History, Progress and the Library have wide
	// layouts of their own, beside the rail.
	const wide = $derived(
		page.route.id === '/(app)/(tabs)' ||
			page.route.id === '/(app)/(tabs)/plan' ||
			page.route.id === '/(app)/(tabs)/plan/cycles/[id]' ||
			page.route.id === '/(app)/(tabs)/journal' ||
			page.route.id === '/(app)/(tabs)/history' ||
			page.route.id === '/(app)/(tabs)/history/progress' ||
			paned(page.route.id)
	);
	// The session builder and New cycle have no rail, and use the whole width.
	const full = $derived(
		page.route.id === '/(app)/templates/new' ||
			page.route.id === '/(app)/templates/[id]/edit' ||
			page.route.id === '/(app)/(tabs)/plan/cycles/new'
	);

	onNavigate((navigation) => {
		if (!document.startViewTransition || kind === 'none') return;
		// The outline already slid in, so the page only fades over it. With
		// Reduce Motion on, every arrival is a fade, as in iOS's own apps.
		const shown = outline || matchMedia('(prefers-reduced-motion: reduce)').matches ? 'fade' : kind;
		document.documentElement.dataset.nav = shown;
		return new Promise((resolve) => {
			const t = document.startViewTransition(async () => {
				resolve();
				await navigation.complete;
			});
			t.finished.finally(() => delete document.documentElement.dataset.nav);
		});
	});
</script>

<EdgeBack />

<!-- iOS Safari applies :active only on a page that listens for touches. -->
<svelte:body ontouchstart={() => {}} />

{#if outline}
	<div
		class="fixed inset-x-0 top-0 z-20 mx-auto h-dvh w-full max-w-[430px] overflow-hidden bg-ground pt-[env(safe-area-inset-top)] arrive-{kind}"
		role="progressbar"
		aria-label="Loading"
	>
		<div class="flex flex-col gap-3.5 px-4 motion-safe:animate-pulse">
			{#if outlineRoot}
				<div class="mt-4 h-9 w-40 rounded-xl bg-well"></div>
				<div class="h-4 w-32 rounded-full bg-well"></div>
			{:else}
				<div class="mt-1.5 h-11 w-28 rounded-full bg-surface shadow-card-sm"></div>
				<div class="mt-2 h-9 w-2/3 rounded-xl bg-well"></div>
				<div class="h-4 w-1/3 rounded-full bg-well"></div>
			{/if}
			<div class="mt-2 h-36 rounded-3xl bg-surface shadow-card"></div>
			<div class="h-24 rounded-3xl bg-surface shadow-card"></div>
			<div class="h-24 rounded-3xl bg-surface shadow-card"></div>
		</div>
	</div>
{/if}

<div
	class="mx-auto min-h-dvh w-full bg-ground text-ink {page.route.id === '/welcome' ? '' : 'max-w-[430px]'} {wide
		? 'lg:max-w-none lg:pl-[calc(6rem+env(safe-area-inset-left))]'
		: full
			? 'lg:max-w-none'
			: ''}"
>
	{@render children()}
</div>
