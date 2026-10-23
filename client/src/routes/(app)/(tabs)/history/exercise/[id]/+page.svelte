<script lang="ts">
	import { page } from '$app/state';
	import Bars from '$lib/Bars.svelte';
	import { shortDate, weekday } from '$lib/dates';
	import GradeDots from '$lib/GradeDots.svelte';
	import Icon from '$lib/Icon.svelte';
	import NavBar from '$lib/NavBar.svelte';
	import { gradeScale, hardest } from '$lib/progress';
	import type { HistorySession } from '$lib/run';

	let { data } = $props();

	// Back to the session or exercise page it was opened from, else History.
	const from = $derived(page.url.searchParams.get('from')?.match(/^\/(history|exercises)\/[\w-]+$/)?.[0]);
	const back = $derived(
		from ? { href: from, label: from.startsWith('/history') ? 'Session' : 'Exercise' } : { href: '/history', label: 'History' }
	);

	// Oldest first for the graph, the last twelve sessions at most.
	const recent = $derived([...data.sessions].slice(0, 12).reverse());
	const ticks = $derived(recent.map((s, i) => (i === 0 || i === recent.length - 1 ? shortDate(s.local_date) : '')));
	const climbs = $derived(data.sessions.flatMap((s) => s.climbs));
	const scale = $derived(gradeScale(climbs, data.scales));
	const loaded = $derived(data.sessions.some((s) => s.sets.some((x) => x.weight_kg)));
	const timed = $derived(!loaded && data.sessions.some((s) => s.sets.some((x) => x.seconds || x.rep_seconds?.length)));

	const heaviest = (s: HistorySession) => Math.max(0, ...s.sets.map((x) => x.weight_kg ?? 0));
	const reps = (s: HistorySession) => s.sets.reduce((n, x) => n + (x.reps ?? x.rep_seconds?.length ?? 0), 0);
	const seconds = (s: HistorySession) => s.sets.reduce((n, x) => n + (x.seconds ?? x.rep_seconds?.reduce((a, b) => a + b, 0) ?? 0), 0);

	const graph = $derived(
		scale
			? null
			: loaded
				? { title: 'Heaviest added weight, kg', values: recent.map(heaviest), format: (n: number) => `${n}` }
				: timed
					? { title: 'Time under load, s', values: recent.map(seconds), format: (n: number) => `${n}` }
					: { title: 'Reps', values: recent.map(reps), format: (n: number) => `${n}` }
	);

	const gradePoints = $derived(scale ? recent.map((s) => hardest(s.climbs, scale)) : []);

	function summary(s: HistorySession) {
		if (s.climbs.length) {
			const sent = s.climbs.filter((c) => c.sent);
			const top = scale ? sent.map((c) => c.grade!).sort((a, b) => scale.grades.indexOf(b) - scale.grades.indexOf(a))[0] : null;
			return [`${s.climbs.length} climb${s.climbs.length === 1 ? '' : 's'}`, `${sent.length} sent`, top && `top ${top}`].filter(Boolean).join(' · ');
		}
		const w = heaviest(s);
		const work = reps(s) ? `${reps(s)} reps` : seconds(s) ? `${seconds(s)} s` : '';
		return [`${s.sets.length} set${s.sets.length === 1 ? '' : 's'}`, work, w ? `+${w} kg` : ''].filter(Boolean).join(' · ');
	}
</script>

<svelte:head><title>{data.name}</title></svelte:head>

<NavBar {back} />

<div class="flex flex-col gap-3.5 px-4 pt-2 pb-6">
	<header class="px-1">
		<h1 class="page-title text-balance">{data.name}</h1>
		<p class="mt-2 text-[15px] font-semibold text-ink-2">
			{data.sessions.length} session{data.sessions.length === 1 ? '' : 's'}{#if data.sessions.length}, last {shortDate(data.sessions[0].local_date)}{/if}
		</p>
	</header>

	{#if recent.length > 1}
		<section class="flex flex-col gap-3 rounded-3xl bg-surface px-[18px] pt-4 pb-[18px] shadow-card">
			{#if scale}
				<h2 class="text-[15px] font-bold">Hardest send by session</h2>
				<GradeDots points={gradePoints} grades={scale.grades} {ticks} />
			{:else if graph}
				<h2 class="text-[15px] font-bold">{graph.title}</h2>
				<Bars parts={[{ values: graph.values, cls: 'bg-rest-on-hero' }]} values format={graph.format} {ticks} />
			{/if}
		</section>
	{/if}

	{#if data.sessions.length}
		<section class="rounded-3xl bg-surface px-[18px] pt-4 pb-2 shadow-card">
			<h2 class="mb-1 text-[15px] font-bold">Sessions</h2>
			<ul>
				{#each data.sessions as s (s.run)}
					<li class="list-row">
						<a href="/history/{s.run}" class="flex min-h-16 items-center gap-3.5 py-2.5">
							<span class="flex size-12 shrink-0 flex-col items-center justify-center rounded-2xl bg-well">
								<span class="text-[10px] leading-none font-bold tracking-wide text-ink-2 uppercase">{weekday(s.local_date)}</span>
								<span class="mt-0.5 digits text-xl leading-none">{Number(s.local_date.slice(8))}</span>
							</span>
							<span class="min-w-0 flex-1">
								<span class="block truncate text-[15px] font-bold">{s.name}</span>
								<span class="mt-0.5 block truncate text-xs font-semibold text-ink-2">{shortDate(s.local_date)} · {summary(s)}</span>
							</span>
							<span class="text-ink-3"><Icon name="chevron-right" size="18px" stroke={2.4} /></span>
						</a>
					</li>
				{/each}
			</ul>
		</section>
	{:else}
		<section class="rounded-3xl bg-surface p-5 shadow-card">
			<p class="text-[15px] font-semibold text-ink-2">Nothing logged for this exercise yet.</p>
		</section>
	{/if}
</div>
