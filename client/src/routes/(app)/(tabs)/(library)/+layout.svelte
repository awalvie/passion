<script lang="ts">
	import Segmented from '$lib/Segmented.svelte';
	import { afterNavigate } from '$app/navigation';
	import { page } from '$app/state';
	import { MediaQuery } from 'svelte/reactivity';
	import LibraryExercises from '$lib/LibraryExercises.svelte';
	import Icon from '$lib/Icon.svelte';
	import LibrarySessions from '$lib/LibrarySessions.svelte';
	import { under } from '$lib/tabs';

	let { children } = $props();

	const parts = [
		{ href: '/templates', label: 'Sessions', add: 'New session template' },
		{ href: '/exercises', label: 'Exercises', add: 'New exercise' }
	];

	// From lg the list stays beside a session's or exercise's own page.
	const wide = new MediaQuery('min-width: 64rem');
	const part = $derived(parts.find((p) => under(page.url.pathname, p.href)));
	const list = $derived(parts.find((p) => page.url.pathname === p.href));

	// The pane keeps its own scroll, so another item starts it at the top.
	let pane = $state<HTMLElement>();
	// An item opened from another page or the other list, or a reload, may sit
	// far down the list.
	afterNavigate(({ from, to }) => {
		if (!wide.current || !to) return;
		if (pane && from?.url?.pathname !== to.url.pathname) pane.scrollTop = 0;
		const was = from?.url && parts.find((p) => under(from.url.pathname, p.href));
		if (was !== part) document.querySelector('[aria-current="true"]')?.scrollIntoView({ block: 'center' });
	});

	// A session or exercise page draws its own nav bar with a back pill, and New
	// exercise its form bar.
	const detail = $derived(/\/(\[id\]|new)$/.test(page.route.id ?? ''));
</script>

{#snippet header(p: (typeof parts)[number])}
	<header class="flex items-start justify-between">
		<h1 class="page-title px-1">Library</h1>
		<!-- New exercise beside the list takes the pane's place, and can go back
		     to the one that was open. -->
		<a
			href={p.href === '/exercises' && page.params.id ? `/exercises/new?open=${page.params.id}` : `${p.href}/new`}
			data-sveltekit-replacestate={wide.current && p.href === '/exercises' ? '' : undefined}
			data-sveltekit-noscroll={wide.current && p.href === '/exercises' ? '' : undefined}
			class="flex size-11 shrink-0 items-center justify-center rounded-full bg-surface text-ink shadow-card-sm"
			aria-label={p.add}
			title={p.add}
		>
			<Icon name="plus" size="1.25rem" />
		</a>
	</header>
{/snippet}

{#snippet lists(href: string)}
	{#if href === '/templates'}
		<LibrarySessions templates={page.data.templates} runs={page.data.runs} today={page.data.today} />
	{:else}
		<LibraryExercises exercises={page.data.exercises} />
	{/if}
{/snippet}

{#snippet segmented()}
	<Segmented label="Library" items={parts.map((p) => ({ ...p, on: page.url.pathname.startsWith(p.href) }))} />
{/snippet}

{#if wide.current && part}
	<div class="mx-auto grid max-w-[1280px] grid-cols-[420px_minmax(0,1fr)]">
		<div class="flex flex-col gap-3.5 px-6 pt-3 pb-6">
			{@render header(part)}
			{@render segmented()}
			<!-- A row only swaps the pane, so it neither adds to Back nor scrolls. -->
			<div data-sveltekit-replacestate data-sveltekit-noscroll>
				{@render lists(part.href)}
			</div>
		</div>
		<!-- The pane scrolls on its own, so a long page leaves the list where it
		     was. It reaches into the tabs' bottom padding, which would otherwise
		     lift it at the end of the page. Its padding and the page's own make
		     Journal's gutter. It draws the divider, as it fills the window and a
		     short list does not. -->
		<div bind:this={pane} class="sticky top-0 -mb-8 h-dvh overflow-y-auto border-l border-line">
			<div class="mx-auto flex min-h-full max-w-[640px] flex-col px-4">
				{@render children()}
			</div>
		</div>
	</div>
{:else if detail}
	{@render children()}
{:else}
	<div class="flex flex-col gap-3.5 px-4 pt-[calc(env(safe-area-inset-top)+0.75rem)]">
		{#if list}
			{@render header(list)}
		{/if}
		{@render segmented()}
		<main>
			{#if list}
				{@render lists(list.href)}
			{/if}
			{@render children()}
		</main>
	</div>
{/if}
