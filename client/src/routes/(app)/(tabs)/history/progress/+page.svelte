<script lang="ts">
	import { goto } from '$app/navigation';
	import Bars from '$lib/Bars.svelte';
	import Button from '$lib/Button.svelte';
	import { formatDate, shortDate } from '$lib/dates';
	import DateField from '$lib/DateField.svelte';
	import Dots from '$lib/Dots.svelte';
	import Empty from '$lib/Empty.svelte';
	import Icon from '$lib/Icon.svelte';
	import NavBar from '$lib/NavBar.svelte';
	import Pyramid from '$lib/Pyramid.svelte';
	import {
		boardsOpen,
		counts,
		filterGroups,
		filterName,
		filterQuery,
		flashRate,
		goesPerSend,
		gradeScale,
		hours,
		isBoulder,
		mixOf,
		monthsBack,
		passes,
		perBucket,
		pyramid,
		sendRate,
		type ClimbFilter,
		type FilterGroup,
		type Trend
	} from '$lib/progress';
	import Segmented from '$lib/Segmented.svelte';
	import Sheet from '$lib/Sheet.svelte';
	import { MediaQuery } from 'svelte/reactivity';

	let { data } = $props();

	// From lg the way back and the title sit in the column, as a page title.
	const wide = new MediaQuery('min-width: 64rem');

	const href = (view: string, range: string, filter = data.filter) => `?view=${view}&range=${range}${filterQuery(filter)}`;
	const round = (h: number) => Math.round(h * 10) / 10;
	const pct = (v: number | null) => (v === null ? '–' : `${v}%`);
	// A week still to come counts for nothing in an average.
	const passed = $derived(data.now === -1 ? data.buckets.length : data.now + 1);

	// A tap on any chart picks its week, or month, for every card. It lasts
	// while the range does.
	let chosen = $state<{ key: string; i: number } | null>(null);
	const picked = $derived(chosen?.key === data.span.key && chosen.i < passed ? chosen.i : null);
	function choose(i: number | null) {
		if (i !== null && i >= passed) return;
		chosen = i === null || i === picked ? null : { key: data.span.key, i };
	}
	const names = $derived(
		data.buckets.map((b) => (data.per === 'month' ? formatDate(b.from, { month: 'long', year: 'numeric' }) : `Week of ${shortDate(b.from)}`))
	);
	const ticks = $derived(data.buckets.map((b, i) => (i === picked ? (data.per === 'month' ? b.tick : shortDate(b.from)) : b.tick)));
	const now = $derived(picked ?? data.now);
	const when = $derived(
		picked === null ? '' : formatDate(data.buckets[picked].from, data.per === 'month' ? { month: 'short', year: 'numeric' } : { day: 'numeric', month: 'short' })
	);
	const inPick = <T extends { local_date: string }>(xs: T[]) =>
		picked === null ? xs : xs.filter((x) => data.buckets[picked].from <= x.local_date && x.local_date <= data.buckets[picked].to);
	const charts = $derived({ pick: picked, onpick: choose, names, ticks, now });
	const boulderFill = 'bg-chart';
	const routeFill = 'bg-chart/50';
	const line = (points: (number | null)[]) => points.flatMap((v, i) => (v === null ? [] : [{ i, v, best: true }]));

	// Two ranges sit on the row; Custom opens the rest.
	const suggested = $derived([
		{ label: '10 weeks', key: '10w' },
		...(data.running ? [{ label: 'This cycle', key: `cycle:${data.running.id}` }] : [])
	]);
	const custom = $derived(!suggested.some((s) => s.key === data.span.key));
	const quick = $derived([
		{ label: '1 month', key: monthsBack(1, data.today) },
		{ label: '2 months', key: monthsBack(2, data.today) },
		{ label: '3 months', key: monthsBack(3, data.today) },
		{ label: '6 months', key: monthsBack(6, data.today) },
		{ label: '1 year', key: monthsBack(12, data.today) },
		{ label: data.today.slice(0, 4), key: `${data.today.slice(0, 4)}-01-01:${data.today}` }
	]);
	// A laptop has room for three more ranges on the row.
	const wider = $derived(quick.slice(2, 5));
	const more = $derived(custom && !wider.some((q) => q.key === data.span.key));
	// A span across a new year names both years, or a year back reads as no time.
	const spanDate = (date: string) =>
		data.span.from.slice(0, 4) === data.span.to.slice(0, 4) ? shortDate(date) : formatDate(date, { day: 'numeric', month: 'short', year: 'numeric' });
	let picking = $state(false);
	let from = $state('');
	let to = $state('');

	function openPicker() {
		from = data.span.from;
		to = data.span.to < data.today ? data.span.to : data.today;
		picking = true;
	}

	function pick(key: string) {
		picking = false;
		goto(href(data.view, key), { replaceState: true, noScroll: true });
	}

	const time = $derived(hours(data.runs, data.buckets));
	const totalHours = $derived(time.reduce((a, b) => a + b, 0));
	// Training time by session, most first, so the mix of work shows.
	const bySession = $derived.by(() => {
		const m = new Map<string, { n: number; h: number }>();
		for (const r of inPick(data.runs)) {
			const e = m.get(r.name) ?? { n: 0, h: 0 };
			m.set(r.name, { n: e.n + 1, h: e.h + (r.elapsed_seconds ?? 0) / 3600 });
		}
		return [...m].sort((a, b) => b[1].h - a[1].h);
	});

	const shown = $derived(data.climbs.filter((c) => passes(c, data.filter)));
	const climbs = $derived(counts(shown, data.buckets));
	const sendRates = $derived(perBucket(shown, data.buckets, sendRate));
	const flashRates = $derived(perBucket(shown, data.buckets, flashRate));
	// Where the climbing was, most first.
	const mix = $derived.by(() => {
		const m = new Map<string, number>();
		for (const c of inPick(shown)) m.set(mixOf(c), (m.get(mixOf(c)) ?? 0) + 1);
		return [...m].sort((a, b) => b[1] - a[1]);
	});

	// Boulders and routes apart, as their grades are on different scales.
	const disciplines = $derived(
		[
			{ label: 'Boulders', climbs: inPick(shown).filter(isBoulder) },
			{ label: 'Routes', climbs: inPick(shown).filter((c) => !isBoulder(c)) }
		].flatMap((d) => {
			const scale = gradeScale(d.climbs, data.scales);
			if (!scale) return [];
			const steps = pyramid(d.climbs, scale);
			return [
				{
					...d,
					pyramid: steps,
					sent: steps.reduce((n, r) => n + r.sent, 0),
					// Climbs graded in another system have no place on this scale.
					elsewhere: d.climbs.filter((c) => c.grade_system && c.grade_system !== scale.system).length
				}
			];
		})
	);

	// The tiles of the last four weeks. A phone shows those of its view; a
	// laptop shows them all.
	const tiles = $derived.by(() => {
		const t = data.trends;
		if (!t) return [];
		return [
			{ label: 'Hours a week', t: t.hours, unit: 'h', view: 'time' },
			{ label: 'Climbs a week', t: t.climbs, unit: '', view: 'climbs' },
			{ label: 'Send rate', t: t.sent, unit: '%', view: 'climbs' },
			{ label: 'Flash rate', t: t.flash, unit: '%', view: 'climbs' }
		];
	});

	// change is how far a trend moved, signed. A rate moves in points.
	function change(t: Trend, unit: string): string | null {
		if (t.now === null || t.then === null) return null;
		const d = round(t.now - t.then);
		if (!d) return 'Same';
		const n = Math.abs(d);
		return `${d > 0 ? '+' : '−'}${unit === '%' ? `${n} ${n === 1 ? 'pt' : 'pts'}` : `${n}${unit}`}`;
	}

	let filtering = $state(false);
	let draft = $state<ClimbFilter>({ discipline: [], setting: [], board: [] });
	const groups = Object.keys(filterGroups) as FilterGroup[];

	function openFilter() {
		draft = structuredClone(data.filter);
		filtering = true;
	}

	function toggle(g: FilterGroup, v: string) {
		draft[g] = draft[g].includes(v) ? draft[g].filter((x) => x !== v) : [...draft[g], v];
	}

	// filterBy adds or takes away one pick in a group, or with no pick, clears it.
	function filterBy(g: FilterGroup, v: string) {
		draft = structuredClone(data.filter);
		if (v) toggle(g, v);
		else draft[g] = [];
		applyFilter(draft);
	}

	function applyFilter(f: ClimbFilter) {
		filtering = false;
		goto(href(data.view, data.span.key, { ...f, board: boardsOpen(f) ? f.board : [] }), { replaceState: true, noScroll: true });
	}
</script>

<svelte:head><title>Progress</title></svelte:head>

{#snippet stat(label: string, value: string | number, at = '')}
	<div class="flex flex-col gap-0.5">
		<dt class="text-xs font-semibold text-ink-2">{label}{#if at}{' · '}<span class="text-ink">{at}</span>{/if}</dt>
		<dd class="digits text-[26px] leading-tight">{value}</dd>
	</div>
{/snippet}

{#snippet tile(label: string, t: Trend, unit: string, cls: string)}
	{@const c = change(t, unit)}
	{@const vs = t.weeks.filter((v) => v !== null)}
	<div class="flex min-w-0 flex-col gap-0.5 rounded-3xl bg-surface px-3 py-2.5 shadow-card lg:px-3.5 lg:py-3 {cls}">
		<dt class="truncate text-xs font-semibold text-ink-2">{label}</dt>
		<dd class="flex items-baseline justify-between gap-1.5">
			<span class="digits text-[22px] leading-tight">{t.now === null ? '–' : `${t.now}${unit}`}</span>
			{#if c}
				<span class="text-xs font-bold whitespace-nowrap text-ink-2">{c}</span>
			{/if}
		</dd>
		<!-- On a laptop, the eight weeks behind the figure. -->
		{#if vs.length > 1}
			{@const lo = Math.min(...vs)}
			{@const hi = Math.max(...vs)}
			<svg class="mt-1.5 hidden h-7 w-full lg:block" viewBox="0 0 100 28" preserveAspectRatio="none" aria-hidden="true">
				<polyline
					points={t.weeks.flatMap((v, i) => (v === null ? [] : [`${(i / (t.weeks.length - 1)) * 100},${25 - ((v - lo) / (hi - lo || 1)) * 22}`])).join(' ')}
					class="fill-none stroke-chart"
					stroke-width="2"
					stroke-linejoin="round"
					stroke-linecap="round"
					vector-effect="non-scaling-stroke"
				/>
			</svg>
		{/if}
	</div>
{/snippet}

{#snippet key(cls: string, label: string)}
	<span class="flex items-center gap-1.5"><span class="size-2.5 rounded-full {cls}"></span>{label}</span>
{/snippet}

{#if wide.current}
	<header class="mx-auto flex max-w-[1104px] flex-col items-start gap-3.5 px-10 pt-1.5 pb-3">
		<a
			href="/history"
			data-back
			class="inline-flex h-11 items-center gap-0.5 rounded-full bg-surface pr-4 pl-2.5 text-[15px] font-bold text-ink shadow-card-sm"
		>
			<Icon name="chevron-left" size="1.25rem" stroke={2.4} />History
		</a>
		<h1 class="page-title px-1">Progress</h1>
	</header>
{:else}
	<NavBar back={{ href: '/history', label: 'History' }} title="Progress" />
{/if}

{#snippet pill(label: string, key: string, cls = '')}
	<a
		href={href(data.view, key)}
		data-sveltekit-replacestate
		data-sveltekit-noscroll
		aria-current={data.span.key === key ? 'true' : undefined}
		class="h-11 shrink-0 items-center rounded-full px-4 text-[15px] font-bold whitespace-nowrap {cls} {data.span.key === key
			? 'bg-ink text-ground'
			: 'bg-surface text-ink shadow-card-sm'}"
	>
		{label}
	</a>
{/snippet}

{#snippet dates()}
	<p class="text-xs font-semibold text-ink-2">
		{data.span.cycle && custom ? `${data.span.cycle.name} · ` : ''}{data.span.from === data.span.to ? spanDate(data.span.from) : `${spanDate(data.span.from)} – ${spanDate(data.span.to)}`}
	</p>
{/snippet}

{#if data.fresh}
	<div class="px-4 pt-2 pb-6 lg:mx-auto lg:max-w-[1104px] lg:px-10">
		<div class="lg:max-w-md">
			<Empty title="No sessions yet" text="Finish one to see your hours, climbs and grades.">
				<Button href="/">Start a session</Button>
			</Empty>
		</div>
	</div>
{:else}
	<!-- A phone shows one view at a time. A laptop shows them all, in two columns. -->
	<div class="flex flex-col gap-3.5 px-4 pt-2 pb-6 lg:mx-auto lg:max-w-[1104px] lg:px-10">
		<div class="lg:hidden">
			<Segmented
				label="What to show"
				replace
				items={[
					{ label: 'Time', on: data.view === 'time', href: href('time', data.span.key) },
					{ label: 'Climbs', on: data.view === 'climbs', href: href('climbs', data.span.key) },
					{ label: 'Grade', on: data.view === 'grade', href: href('grade', data.span.key) }
				]}
			/>
		</div>
		<!-- On a laptop the ranges and the picked week share one bar at the top.
		     On a phone the box drops out, so the week bar sticks for the whole page. -->
		<div class="contents lg:sticky lg:flex lg:top-0 lg:z-20 lg:-my-2 lg:flex-row lg:items-center lg:justify-between lg:bg-ground lg:py-2">
			<nav class="flex gap-1.5" aria-label="Range">
				{#each suggested as s (s.key)}{@render pill(s.label, s.key, 'flex')}{/each}
				{#each wider as q (q.key)}{@render pill(q.label, q.key, 'hidden lg:flex')}{/each}
				<button
					type="button"
					class="flex h-11 min-w-0 flex-1 items-center justify-center gap-1.5 rounded-full px-4 text-[15px] font-bold lg:flex-none {custom
						? 'bg-ink text-ground'
						: 'bg-surface text-ink shadow-card-sm'} {more ? '' : 'lg:bg-surface lg:text-ink lg:shadow-card-sm'}"
					aria-current={custom ? 'true' : undefined}
					aria-haspopup="dialog"
					onclick={openPicker}
				>
					<Icon name="calendar" size="1rem" />
					<span class="truncate">Custom</span>
				</button>
			</nav>
			{#if picked !== null}
				<!-- Under the nav bar, so the week stays named while its cards scroll by. -->
				<div class="sticky top-[calc(3.5rem+env(safe-area-inset-top))] z-20 -my-1.5 bg-ground py-1.5 lg:static lg:my-0 lg:py-0">
					<div class="flex w-fit max-w-full items-center rounded-full bg-ink px-0.5 text-ground">
						<button
							type="button"
							class="flex size-11 shrink-0 items-center justify-center rounded-full disabled:opacity-40"
							aria-label="Previous {data.per}"
							disabled={picked === 0}
							onclick={() => choose(picked! - 1)}><Icon name="chevron-left" size="1.25rem" stroke={2.4} /></button
						>
						<span class="truncate px-1.5 text-[15px] font-bold" aria-live="polite">{names[picked]}</span>
						<button
							type="button"
							class="flex size-11 shrink-0 items-center justify-center rounded-full disabled:opacity-40"
							aria-label="Next {data.per}"
							disabled={picked === passed - 1}
							onclick={() => choose(picked! + 1)}><Icon name="chevron-right" size="1.25rem" stroke={2.4} /></button
						>
						<button type="button" class="flex size-11 shrink-0 items-center justify-center rounded-full" aria-label="Show the whole range" onclick={() => choose(null)}
							><Icon name="x" size="1.25rem" stroke={2.4} /></button
						>
					</div>
				</div>
			{/if}
		</div>
		{#if tiles.length}
			<h2 class="mt-3 hidden px-1 text-xl font-bold tracking-tight lg:block">
				Last 4 weeks <span class="text-[15px] font-semibold text-ink-2">against the 4 before</span>
			</h2>
			<dl class="grid auto-cols-fr grid-flow-col gap-2 lg:grid-flow-row lg:grid-cols-4 lg:gap-3.5 {data.view === 'grade' ? 'max-lg:hidden' : ''}" aria-label="Last 4 weeks, against the 4 before">
				{#each tiles as t (t.label)}{@render tile(t.label, t.t, t.unit, t.view === data.view ? '' : 'max-lg:hidden')}{/each}
			</dl>
		{/if}

		<div class="flex-col gap-3.5 lg:mt-3 lg:flex {data.view === 'time' ? 'flex' : 'hidden'}">
			<div class="mt-1 flex flex-col px-1">
				<h2 class="text-xl font-bold tracking-tight">All sessions</h2>
				{@render dates()}
			</div>
			<div class="flex flex-col gap-3.5 lg:grid lg:grid-cols-6">
				<section class="flex flex-col gap-4 rounded-3xl bg-surface px-[18px] pt-4 pb-[18px] shadow-card lg:col-span-4">
					<dl class="grid grid-cols-3 gap-3">
						{@render stat('Hours', round(picked === null ? totalHours : time[picked]), when)}
						{@render stat(`A ${data.per}`, round(totalHours / passed))}
						{@render stat('Sessions', inPick(data.runs).length)}
					</dl>
					<Bars parts={[{ values: time, cls: 'bg-chart' }]} format={(h) => `${round(h)}h`} {...charts} />
				</section>
				{#if bySession.length}
					<section class="rounded-3xl bg-surface px-[18px] pt-4 pb-2 shadow-card lg:col-span-2">
						<h2 class="mb-1 text-[15px] font-bold">By session</h2>
						<ul>
							{#each bySession as [name, s] (name)}
								<li class="flex items-center gap-3 py-3 list-row">
									<span class="min-w-0 flex-1 truncate text-[15px] font-bold">{name}</span>
									<span class="text-xs font-semibold text-ink-2">{s.n}×</span>
									<span class="w-12 text-right digits text-[15px]">{round(s.h)}h</span>
								</li>
							{/each}
						</ul>
					</section>
				{/if}
			</div>
		</div>

		<div class="flex-col gap-3.5 lg:mt-3 lg:flex {data.view === 'time' ? 'hidden' : 'flex'}">
			<div class="mt-1 flex flex-col px-1">
				<button type="button" class="-my-2 flex min-h-11 items-center gap-1 self-start text-left text-xl font-bold tracking-tight lg:hidden" aria-haspopup="dialog" onclick={openFilter}>
					{filterName(data.filter)}
					<Icon name="chevron-down" size="1.125rem" stroke={2.6} />
				</button>
				<h2 class="hidden text-xl font-bold tracking-tight lg:block">{filterName(data.filter)}</h2>
				{@render dates()}
			</div>
			<!-- A laptop has room to show the filter itself in place of its sheet. -->
			<div class="hidden flex-wrap items-end gap-x-5 gap-y-2.5 lg:flex">
				{#each groups as g (g)}
					{@const closed = g === 'board' && !boardsOpen(data.filter)}
					<div class="flex flex-col gap-1.5">
						<span class="px-1 text-xs font-semibold {closed ? 'text-ink-3' : 'text-ink-2'}">{filterGroups[g].label}</span>
						<div class="flex gap-1 rounded-full bg-well p-1" role="group" aria-label={filterGroups[g].label}>
							{#each [['', 'All'], ...Object.entries(filterGroups[g].options)] as [v, label] (v)}
								{@const on = !closed && (v ? data.filter[g].includes(v) : !data.filter[g].length)}
								<button
									type="button"
									class="flex h-11 items-center rounded-full px-4 text-[15px] font-bold disabled:opacity-40 {on ? 'bg-surface text-ink shadow-card-sm dark:bg-ink/15' : 'text-ink-2'}"
									aria-pressed={on}
									disabled={closed}
									onclick={() => filterBy(g, v)}
									>{label}</button
								>
							{/each}
						</div>
					</div>
				{/each}
			</div>
			{#if !shown.length}
				<section class="flex flex-col items-start gap-1 rounded-3xl bg-surface p-5 shadow-card">
					{#if filterQuery(data.filter) && data.climbs.length}
						<p class="text-[15px] font-semibold text-ink-2">No climbs match in this range.</p>
						<button type="button" class="-mb-3 flex min-h-11 items-center gap-0.5 text-[15px] font-bold text-ink" onclick={() => applyFilter({ discipline: [], setting: [], board: [] })}>
							Show all climbs<Icon name="chevron-right" size="16px" />
						</button>
					{:else}
						<p class="text-[15px] font-semibold text-ink-2">No climbs in this range.</p>
					{/if}
				</section>
			{:else}
				<!-- The phone's order is climbs, sent, flash, where. A laptop puts where beside climbs. -->
				<div class="flex-col gap-3.5 lg:grid lg:grid-cols-6 {data.view === 'climbs' ? 'flex' : 'hidden'}">
					<section class="flex flex-col gap-4 rounded-3xl bg-surface px-[18px] pt-4 pb-[18px] shadow-card lg:col-span-4 lg:row-start-1">
						<dl class="grid grid-cols-3 gap-3">
							{@render stat('Climbs', inPick(shown).length, when)}
							{@render stat(`A ${data.per}`, round(shown.length / passed))}
							{@render stat('Tries per send', goesPerSend(inPick(shown)) ?? '–')}
						</dl>
						<Bars
							parts={[
								{ values: climbs.map((c) => c.boulders), cls: boulderFill },
								{ values: climbs.map((c) => c.routes), cls: routeFill }
							]}
							{...charts}
						/>
						<div class="flex gap-4 text-xs font-semibold text-ink-2">
							{#if shown.some(isBoulder)}{@render key(boulderFill, 'Boulders')}{/if}
							{#if shown.some((c) => !isBoulder(c))}{@render key(routeFill, 'Routes')}{/if}
						</div>
					</section>
					<section class="flex flex-col gap-4 rounded-3xl bg-surface px-[18px] pt-4 pb-[18px] shadow-card lg:col-span-3">
						<dl>{@render stat('Send rate', pct(sendRate(inPick(shown))), when)}</dl>
						<Dots n={data.buckets.length} marks={line(sendRates)} label={(v) => `${v}%`} zero {...charts} />
					</section>
					<section class="flex flex-col gap-4 rounded-3xl bg-surface px-[18px] pt-4 pb-[18px] shadow-card lg:col-span-3">
						<dl>{@render stat('Flash rate', pct(flashRate(inPick(shown))), when)}</dl>
						<Dots n={data.buckets.length} marks={line(flashRates)} label={(v) => `${v}%`} zero {...charts} />
					</section>
					<section class="rounded-3xl bg-surface px-[18px] pt-4 pb-3 shadow-card lg:col-span-2 lg:col-start-5 lg:row-start-1">
						<h2 class="mb-2 text-[15px] font-bold">Where</h2>
						<ul class="flex flex-col gap-3">
							{#each mix as [where, n] (where)}
								<li class="flex flex-col gap-1.5">
									<span class="flex items-baseline justify-between gap-3 text-[15px] font-bold">
										{where}
										<span class="digits">{n}</span>
									</span>
									<span class="h-2 rounded-full bg-well"><span class="block h-full rounded-full bg-chart" style:width="{(n / mix[0][1]) * 100}%"></span></span>
								</li>
							{/each}
						</ul>
					</section>
				</div>
				<div class="flex-col gap-3.5 lg:grid lg:grid-cols-2 {data.view === 'grade' ? 'flex' : 'hidden'}">
					{#each disciplines as d (d.label)}
						<section class="flex flex-col gap-3 rounded-3xl bg-surface px-[18px] pt-4 pb-[18px] shadow-card">
							<dl>{@render stat(`${d.label} sent`, d.sent, when)}</dl>
							{#if d.pyramid.length}
								<Pyramid rows={d.pyramid} />
								<div class="flex gap-4 text-xs font-semibold text-ink-2">
									{@render key('bg-chart', 'Flash')}
									{@render key('bg-chart/50', 'Sent after a try')}
								</div>
							{/if}
							{#if d.elsewhere}
								<p class="text-xs font-semibold text-ink-2">
									{d.elsewhere}
									{d.elsewhere === 1 ? 'climb' : 'climbs'} graded in another system not shown.
								</p>
							{/if}
						</section>
					{:else}
						<section class="rounded-3xl bg-surface p-5 shadow-card">
							<p class="text-[15px] font-semibold text-ink-2">No graded climbs in this range.</p>
						</section>
					{/each}
				</div>
			{/if}
		</div>
	</div>
{/if}

<Sheet bind:open={picking} title="Range">
	<div class="flex flex-col gap-3.5">
		<div class="grid grid-cols-3 gap-1.5">
			{#each quick as q (q.label)}
				<button
					type="button"
					class="flex h-11 items-center justify-center rounded-full text-[15px] font-bold {data.span.key === q.key
						? 'bg-ink text-ground'
						: 'bg-surface text-ink shadow-card-sm'}"
					aria-pressed={data.span.key === q.key}
					onclick={() => pick(q.key)}>{q.label}</button
				>
			{/each}
		</div>
		{#if data.cycles.length}
			<h2 class="mt-1 px-1 text-[15px] font-bold">Cycles</h2>
			<ul class="rounded-3xl bg-surface px-[18px] py-1 shadow-card">
				{#each data.cycles as c (c.id)}
					<li class="list-row">
						<button type="button" class="flex min-h-[52px] w-full items-center gap-3 py-2 text-left" onclick={() => pick(`cycle:${c.id}`)}>
							<span class="min-w-0 flex-1 truncate text-[15px] font-bold">{c.name}</span>
							<span class="text-xs font-semibold text-ink-2">{shortDate(c.starts)} – {shortDate(c.ends)}</span>
						</button>
					</li>
				{/each}
			</ul>
		{/if}
		<h2 class="mt-1 px-1 text-[15px] font-bold">Dates</h2>
		<div class="flex flex-col gap-3.5 rounded-3xl bg-surface p-[18px] shadow-card">
			<div class="grid grid-cols-2 gap-3">
				<label class="flex flex-col gap-1.5 text-xs font-semibold text-ink-2">
					From
					<DateField bind:value={from} label="From" min={data.first} max={data.today} />
				</label>
				<label class="flex flex-col gap-1.5 text-xs font-semibold text-ink-2">
					To
					<DateField bind:value={to} label="To" min={from} max={data.today} />
				</label>
			</div>
			<Button onclick={() => pick(`${from}:${to}`)} disabled={!from || !to || to < from || from > data.today}>Show</Button>
		</div>
	</div>
</Sheet>

<Sheet bind:open={filtering} title="Show">
	<div class="flex flex-col gap-3.5">
		{#each groups as g (g)}
			{@const closed = g === 'board' && !boardsOpen(draft)}
			<h2 class="mt-1 px-1 text-[15px] font-bold {closed ? 'text-ink-3' : ''}">{filterGroups[g].label}</h2>
			<div class="flex flex-wrap gap-1.5">
				{#each Object.entries(filterGroups[g].options) as [v, label] (v)}
					{@const on = !closed && draft[g].includes(v)}
					<button
						type="button"
						class="flex h-11 items-center rounded-full px-4 text-[15px] font-bold disabled:opacity-40 {on ? 'bg-ink text-ground' : 'bg-surface text-ink shadow-card-sm'}"
						aria-pressed={on}
						disabled={closed}
						onclick={() => toggle(g, v)}>{label}</button
					>
				{/each}
			</div>
		{/each}
	</div>
	{#snippet footer()}
		<div class="grid grid-cols-[1fr_2fr] items-center gap-2">
			<Button variant="secondary" onclick={() => applyFilter({ discipline: [], setting: [], board: [] })}>All climbs</Button>
			<Button onclick={() => applyFilter(draft)}>Apply</Button>
		</div>
	{/snippet}
</Sheet>
