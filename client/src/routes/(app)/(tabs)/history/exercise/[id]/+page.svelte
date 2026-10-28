<script lang="ts">
	import { page } from '$app/state';
	import ExerciseHistory from '$lib/ExerciseHistory.svelte';
	import NavBar from '$lib/NavBar.svelte';
	import { progressFrom } from '$lib/progress';
	import { plural } from '$lib/text';

	let { data } = $props();

	// Back to the session, exercise or Progress page it was opened from, else History.
	const from = $derived(page.url.searchParams.get('from')?.match(/^\/(history|exercises)\/[\w-]+$/)?.[0]);
	const progress = $derived(progressFrom(page.url.searchParams.get('from')));
	const back = $derived(
		progress
			? { href: progress, label: 'Progress' }
			: from
				? { href: from, label: from.startsWith('/history') ? 'Session' : 'Exercise' }
				: { href: '/history', label: 'History' }
	);
</script>

<svelte:head><title>{data.name}</title></svelte:head>

<NavBar {back} />

<div class="flex flex-col gap-3.5 px-4 pt-2 pb-6">
	<header class="px-1">
		<h1 class="page-title text-balance">{data.name}</h1>
		<p class="mt-2 text-[15px] font-semibold text-ink-2">{plural(data.sessions.length, 'session')}</p>
	</header>

	<ExerciseHistory sessions={data.sessions} scales={data.scales} sides={data.perSide ? 2 : 1} href={(s) => `/history/${s.run}`} />
</div>
