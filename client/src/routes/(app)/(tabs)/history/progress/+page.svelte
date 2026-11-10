<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import Bars from '$lib/Bars.svelte';
	import Button from '$lib/Button.svelte';
	import { formatDate, shortDate } from '$lib/dates';
	import DateField from '$lib/DateField.svelte';
	import Dots from '$lib/Dots.svelte';
	import Empty from '$lib/Empty.svelte';
	import Icon from '$lib/Icon.svelte';
	import Info from '$lib/Info.svelte';
	import NavBar from '$lib/NavBar.svelte';
	import Pyramid from '$lib/Pyramid.svelte';
	import {
		boardsOpen,
		climbCount,
		counts,
		figure,
		filterGroups,
		filterName,
		filterQuery,
		flashRate,
		gradeScale,
		hours,
		isBoulder,
		grades as topGrades,
		lifts,
		mixOf,
		monthsBack,
		passes,
		perBucket,
		pyramid,
		sendRate,
		type ClimbFilter,
		type FilterGroup,
		type Lift
	} from '$lib/progress';
	import Segmented from '$lib/Segmented.svelte';
	import Tabs from '$lib/Tabs.svelte';
	import TrendTile from '$lib/TrendTile.svelte';
	import Sheet from '$lib/Sheet.svelte';
	import { plural } from '$lib/text';
	import { MediaQuery } from 'svelte/reactivity';

	let { data } = $props();

	// From lg the way back and the title sit in the column, as a page title.
	const wide = new MediaQuery('min-width: 64rem');

	// Opened from Today, the way back leads there, through every view and range.
	const fromToday = $derived(page.url.searchParams.get('from') === '/');
	const back = $derived(fromToday ? { href: '/', label: 'Today' } : { href: '/history', label: 'History' });
	const href = (view: string, range: string, filter = data.filter, unique = data.unique, tried = data.tried) =>
		`?view=${view}&range=${range}${filterQuery(filter)}${unique ? '&count=unique' : ''}${tried ? '&pyramid=tried' : ''}${fromToday ? '&from=/' : ''}`;
	const round = (h: number) => Math.round(h * 10) / 10;
	const pct = (v: number | null) => (v === null ? '–' : `${v}%`);
	// A week still to come counts for nothing in an average.
	const passed = $derived(data.now === -1 ? data.buckets.length : data.now + 1);
	// A phone shows one view at a time; a laptop shows them all.
	const on = (...views: string[]) => views.includes(data.view);
	// An average is over the full weeks, or months, from the first session's;
	// the one under way shows in the bars only.
	const full = $derived(data.buckets.flatMap((b, i) => (b.to < data.today && b.to >= data.first ? [i] : [])));
	const mean = (values: number[]) => (full.length ? full.reduce((n, i) => n + values[i], 0) / full.length : undefined);
	const average = (values: number[]) => {
		const m = mean(values);
		return m === undefined ? '–' : round(m);
	};
	const perInfo = $derived(
		`A ${data.per} is the average over the full ${data.per}s of the range, from your first session's. The ${data.per} under way shows in the bars but not in the average.`
	);

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
	// A picked week names its date, and its neighbours drop theirs so the two do not overprint.
	const ticks = $derived(
		data.buckets.map((b, i) => (picked === null ? b.tick : i === picked ? (data.per === 'month' ? b.tick : shortDate(b.from)) : Math.abs(i - picked) === 1 ? '' : b.tick))
	);
	const now = $derived(picked ?? data.now);
	const when = $derived(
		picked === null ? '' : formatDate(data.buckets[picked].from, data.per === 'month' ? { month: 'short', year: 'numeric' } : { day: 'numeric', month: 'short' })
	);
	const inPick = <T extends { local_date: string }>(xs: T[]) =>
		picked === null ? xs : xs.filter((x) => data.buckets[picked].from <= x.local_date && x.local_date <= data.buckets[picked].to);
	const charts = $derived({ pick: picked, onpick: choose, names, ticks, now });
	const pill = 'flex h-11 items-center gap-1 rounded-full bg-surface pr-3.5 pl-4 text-[15px] font-bold text-ink shadow-card-sm';
	// Pale green is routes across the Climbs cards: the chart, its key and Where.
	const boulderFill = 'bg-chart';
	const routeFill = 'bg-chart';
	const line = (points: (number | null)[]) => points.flatMap((v, i) => (v === null ? [] : [{ i, v, best: true }]));

	// Two ranges sit on the row; Custom opens the rest.
	const suggested = $derived([
		{ label: '10 weeks', key: '10w' },
		...(data.running ? [{ label: 'This cycle', key: `cycle:${data.running.id}` }] : [])
	]);
	const custom = $derived(!suggested.some((s) => s.key === data.span.key));
	const quick = $derived([
		{ label: '10 weeks', key: '10w' },
		{ label: '1 month', key: monthsBack(1, data.today) },
		{ label: '3 months', key: monthsBack(3, data.today) },
		{ label: '6 months', key: monthsBack(6, data.today) },
		{ label: '1 year', key: monthsBack(12, data.today) },
		{ label: data.today.slice(0, 4), key: `${data.today.slice(0, 4)}-01-01:${data.today}` }
	]);
	// A span across a new year names both years, or a year back reads as no time.
	const spanDate = (date: string) =>
		data.span.from.slice(0, 4) === data.span.to.slice(0, 4) ? shortDate(date) : formatDate(date, { day: 'numeric', month: 'short', year: 'numeric' });
	const spanDates = $derived(data.span.from === data.span.to ? spanDate(data.span.from) : `${spanDate(data.span.from)} – ${spanDate(data.span.to)}`);
	const named = $derived(data.span.cycle ? data.span.cycle.name : data.span.key === '4w' ? 'Last 4 weeks' : quick.find((q) => q.key === data.span.key)?.label);
	// A custom span is named by its dates.
	const rangeName = $derived(named ?? spanDates);
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
		choose(null);
		goto(href(data.view, key), { replaceState: true, noScroll: true });
	}

	const time = $derived(hours(data.runs, data.buckets));
	const totalHours = $derived(time.reduce((a, b) => a + b, 0));
	// Training time by session, most first, so the mix of work shows.
	const bySession = $derived.by(() => {
		const m = new Map<string, { name: string; n: number; h: number }>();
		for (const r of inPick(data.runs)) {
			const k = r.template ?? r.name;
			const e = m.get(k) ?? { name: r.name, n: 0, h: 0 };
			m.set(k, { name: e.name, n: e.n + 1, h: e.h + (r.elapsed_seconds ?? 0) / 3600 });
		}
		return [...m].sort((a, b) => b[1].h - a[1].h);
	});
	let allSessions = $state(false);

	const shown = $derived(data.climbs.filter((c) => passes(c, data.filter)));
	const climbs = $derived(counts(shown, data.buckets, data.unique));
	const split = $derived([
		{ label: 'Boulders', values: climbs.map((c) => c.boulders) },
		{ label: 'Routes', values: climbs.map((c) => c.routes) }
	]);
	const splitTop = $derived(Math.max(...climbs.map((c) => Math.max(c.boulders, c.routes)), 0));
	const sendRates = $derived(perBucket(shown, data.buckets, sendRate));
	const flashRates = $derived(perBucket(shown, data.buckets, flashRate));
	// Where the climbing was, most first.
	const mix = $derived.by(() => {
		const m = new Map<string, number>();
		for (const c of inPick(shown)) m.set(mixOf(c), (m.get(mixOf(c)) ?? 0) + climbCount([c], data.unique));
		return [...m].sort((a, b) => b[1] - a[1]);
	});

	const strength = $derived(lifts(data.sets, data.buckets));
	// A lift reads its best set: the latest session's, or the picked bucket's.
	function lift(l: Lift): { main: string; small: string; note: string } {
		const x = picked === null ? l.latest : l.best[picked];
		if (!x) return { main: '–', small: '', note: 'Not done' };
		const [main, small] = figure(x, l.measure);
		if (picked !== null) return { main, small, note: '' };
		const c = l.change;
		const note = c === null ? '' : c === 0 ? 'Same' : `${c > 0 ? '+' : '−'}${Math.abs(c)} ${l.measure}`;
		return { main, small, note };
	}
	const liftHref = (l: Lift) =>
		`/history/exercise/${l.exercise}?name=${encodeURIComponent(l.name)}&from=${encodeURIComponent(page.url.pathname + page.url.search)}`;

	// Boulders and routes apart, as their grades are on different scales. A
	// picked week's bars are measured against the whole range's.
	const disciplines = $derived(
		[
			{ label: 'Boulders', range: shown.filter(isBoulder) },
			{ label: 'Routes', range: shown.filter((c) => !isBoulder(c)) }
		].flatMap(({ label, range }) => {
			const picks = inPick(range);
			const scale = gradeScale(picks, data.scales);
			if (!scale) return [];
			const by = data.tried ? 'tried' : 'sent';
			const steps = pyramid(picks, scale, by);
			const tops = topGrades(range, data.buckets, scale);
			const at = (picked === null ? tops : [tops[picked]]).filter((t): t is number => t !== null);
			return [
				{
					label,
					scale,
					tops,
					top: at.length ? scale.grades[Math.max(...at)] : '–',
					pyramid: steps,
					most: Math.max(...pyramid(range, scale, by).map((r) => r[by])),
					sent: steps.reduce((n, r) => n + r.sent, 0),
					tried: steps.reduce((n, r) => n + r.tried, 0),
					// Climbs graded in another system have no place on this scale.
					elsewhere: picks.filter((c) => c.grade_system && c.grade_system !== scale.system).length
				}
			];
		})
	);

	// The tiles of the last four weeks. A phone shows them over Time only, the
	// view a glance starts from, so they do not push every view down; a laptop
	// shows them over everything.
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

	function applyFilter(f: ClimbFilter, unique = data.unique, tried = data.tried) {
		filtering = false;
		goto(href(data.view, data.span.key, { ...f, board: boardsOpen(f) ? f.board : [] }, unique, tried), { replaceState: true, noScroll: true });
	}
</script>

<svelte:head><title>Progress</title></svelte:head>

{#snippet stat(label: string, value: string | number)}
	<div class="flex flex-col gap-0.5">
		<dt class="text-xs font-semibold text-ink-2">{label}</dt>
		<dd class="digits text-[26px] leading-tight">{value}</dd>
	</div>
{/snippet}

{#snippet counting()}
	<Segmented
		label="Count climbs"
		replace
		items={[
			{ label: 'Every go', on: !data.unique, href: href(data.view, data.span.key, data.filter, false) },
			{ label: 'Once a session', on: data.unique, href: href(data.view, data.span.key, data.filter, true) }
		]}
	/>
{/snippet}

{#snippet key(cls: string, label: string)}
	<span class="flex items-center gap-1.5"><span class="size-2.5 rounded-full {cls}"></span>{label}</span>
{/snippet}

{#if wide.current}
	<header class="mx-auto flex max-w-[1104px] flex-col items-start gap-3.5 px-10 pt-1.5 pb-3">
		<a
			href={back.href}
			data-back
			class="inline-flex h-11 items-center gap-0.5 rounded-full bg-surface pr-4 pl-2.5 text-[15px] font-bold text-ink shadow-card-sm"
		>
			<Icon name="chevron-left" size="1.25rem" stroke={2.4} />{back.label}
		</a>
		<h1 class="page-title px-1">Progress</h1>
	</header>
{:else}
	<NavBar {back} title="Progress" />
{/if}

<!-- The range is a pill: a tap opens its sheet. A picked week adds a chip beside it,
     with its own ×, so the range stays in reach. On a phone the climb filter is a pill
     beside it, and the dates go on a line below. -->
{#snippet rangeLine()}
	<div class="flex flex-wrap items-center gap-x-2.5 gap-y-2 lg:flex-nowrap lg:gap-x-4">
		<button type="button" class="{pill} min-w-0" aria-haspopup="dialog" onclick={openPicker}>
			<span class="truncate">{rangeName}</span><Icon name="chevron-down" size="1rem" stroke={2.6} />
		</button>
		{#if on('climbs', 'grade')}
			<button type="button" class="{pill} min-w-0 lg:hidden" aria-haspopup="dialog" onclick={openFilter}>
				<span class="truncate">{filterName(data.filter)}</span><Icon name="chevron-down" size="1rem" stroke={2.6} />
			</button>
		{/if}
		{#if picked !== null}
			<span class="flex h-11 min-w-0 items-center gap-0.5 rounded-full pr-1.5 pl-4 segment-on">
				<span class="truncate text-[15px] font-bold" aria-live="polite">{names[picked]}</span>
				<button
					type="button"
					class="relative flex size-8 shrink-0 items-center justify-center rounded-full before:absolute before:-inset-1.5 before:content-['']"
					aria-label="Show the whole range"
					onclick={() => choose(null)}><Icon name="x" size="1rem" stroke={2.6} /></button
				>
			</span>
		{/if}
		{#if named}<p class="w-full min-w-0 truncate px-1 text-[15px] font-semibold text-ink-2 lg:w-auto">{spanDates}</p>{/if}
	</div>
{/snippet}

{#snippet sentTried()}
	<Segmented
		label="Pyramid shows"
		replace
		items={[
			{ label: 'Sent', on: !data.tried, href: href(data.view, data.span.key, data.filter, data.unique, false) },
			{ label: 'Tried', on: data.tried, href: href(data.view, data.span.key, data.filter, data.unique, true) }
		]}
	/>
{/snippet}

<!-- How the last 4 weeks went, whatever the range: a glance before the detail. -->
{#snippet lastWeeks()}
	{#if tiles.some((t) => t.t.now !== null)}
		<section class="flex flex-col gap-1">
			<div class="relative flex items-center justify-between gap-3 px-1">
				<h2 class="text-xl font-bold tracking-tight">Last 4 weeks</h2>
				<Info text="Each figure is a week on average over the last 4 weeks, and its change is on the 4 weeks before. The bars are the last 4 weeks, and the dashed line is the 4 weeks before." />
			</div>
			<ul class="grid grid-cols-2 gap-2.5 lg:grid-cols-4 lg:gap-3.5">
				{#each tiles.filter((t) => t.t.now !== null) as t (t.label)}<TrendTile label={t.label} t={t.t} unit={t.unit} href={wide.current ? undefined : href(t.view, data.span.key)} replace />{/each}
			</ul>
		</section>
	{/if}
{/snippet}

{#snippet heading(title: string, cls = '')}
	<h2 class="px-1 text-xl font-bold tracking-tight {cls}">{title}</h2>
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
		{#if wide.current}{@render lastWeeks()}{/if}
		<!-- The view picks a question; it is navigation, so it is drawn as History's Tabs, above the quieter range and filter. -->
		<div class="lg:hidden">
			<Tabs
				label="What to show"
				bind:value={() => data.view, (view) => goto(href(view, data.span.key), { replaceState: true, noScroll: true, keepFocus: true })}
				options={[
					{ value: 'time', label: 'Time' },
					{ value: 'climbs', label: 'Climbs' },
					{ value: 'grade', label: 'Grades' },
					{ value: 'strength', label: 'Strength' }
				]}
			/>
		</div>
		<!-- On a phone the tiles sit under the strip, so the strip keeps its place on every view. -->
		{#if !wide.current && data.view === 'time'}{@render lastWeeks()}{/if}
		<!-- On a laptop the range sticks at the top. -->
		<div class="lg:sticky lg:top-0 lg:z-20 lg:-mb-2 lg:mt-1 lg:bg-ground lg:py-2">{@render rangeLine()}</div>

		<div class="flex-col gap-3.5 lg:mt-1 lg:flex {on('time') ? 'flex' : 'hidden'}">
			{@render heading('Time', 'max-lg:hidden')}
			<!-- By session goes under the chart, not beside it: its length follows the kinds of
			     session, so beside a chart it left a gap under one or the other. -->
			<div class="flex flex-col gap-3.5">
				<section class="flex flex-col gap-4 rounded-3xl bg-surface px-[18px] pt-4 pb-[18px] shadow-card">
					<div class="relative flex items-start justify-between gap-3">
						<dl class="grid flex-1 grid-cols-3 items-end gap-3 lg:max-w-xl">
							{@render stat('Hours', round(picked === null ? totalHours : time[picked]))}
							{@render stat(`Hours a ${data.per}`, average(time))}
							{@render stat('Sessions', inPick(data.runs).length)}
						</dl>
						<Info text={perInfo} />
					</div>
					<Bars parts={[{ values: time, cls: 'bg-chart' }]} format={(h) => `${round(h)}h`} {...charts} />
				</section>
				{#if bySession.length}
					<section class="rounded-3xl bg-surface px-[18px] pt-4 pb-2 shadow-card">
						<h3 class="mb-1 text-[15px] font-bold">By session</h3>
						<ul class="lg:grid lg:grid-cols-2 lg:gap-x-9">
							{#each allSessions ? bySession : bySession.slice(0, 6) as [key, s] (key)}
								<li class="flex items-center gap-3 py-3 list-row lg:[&:nth-child(2)]:shadow-none">
									<span class="min-w-0 flex-1 truncate text-[15px] font-semibold">{s.name}</span>
									<span class="text-xs font-semibold text-ink-2">{s.n}×</span>
									<span class="w-12 text-right digits text-[15px]">{round(s.h)}h</span>
								</li>
							{/each}
						</ul>
						{#if bySession.length > 6}
							<button type="button" class="mt-1 mb-1 flex h-11 items-center text-[15px] font-bold text-ink" onclick={() => (allSessions = !allSessions)}>
								{allSessions ? 'Show fewer' : `Show all ${bySession.length}`}
							</button>
						{/if}
					</section>
				{/if}
			</div>
		</div>

		<div class="flex-col gap-3.5 lg:mt-3 lg:flex {on('climbs', 'grade') ? 'flex' : 'hidden'}">
			<div class="hidden items-center gap-5 px-1 lg:flex">
				<h2 class="text-xl font-bold tracking-tight">Climbs</h2>
				<div class="w-72 shrink-0">{@render counting()}</div>
			</div>
			<!-- A laptop has room to show the filter itself in place of its sheet. -->
			<!-- Its options name each group, so the groups need no labels. -->
			<div class="hidden flex-wrap items-center gap-x-5 gap-y-2.5 lg:flex">
				{#each groups as g (g)}
					{@const closed = g === 'board' && !boardsOpen(data.filter)}
					<div>
						<div class="flex gap-1 rounded-full bg-well p-1" role="group" aria-label={filterGroups[g].label}>
							{#each [['', 'All'], ...Object.entries(filterGroups[g].options)] as [val, label] (val)}
								{@const pressed = !closed && (val ? data.filter[g].includes(val) : !data.filter[g].length)}
								<button
									type="button"
									class="flex h-11 items-center rounded-full px-4 text-[15px] font-bold disabled:opacity-40 {pressed ? 'segment-on' : 'text-ink-2'}"
									aria-pressed={pressed}
									disabled={closed}
									onclick={() => filterBy(g, val)}
									>{label}</button
								>
							{/each}
						</div>
					</div>
				{/each}
			</div>
			{#if !shown.length}
				{#if filterQuery(data.filter) && data.climbs.length}
					<Empty title="No climbs match" text="None of this range's climbs fit the filter." onclear={() => applyFilter({ discipline: [], setting: [], board: [] })} />
				{:else}
					<Empty title="No climbs" text="Log climbs in a session to see them here." />
				{/if}
			{:else}
				<!-- On a phone the count sits over the cards it changes, in reach. -->
				<div class="lg:hidden {on('climbs') ? '' : 'hidden'}">{@render counting()}</div>
				<!-- Climbs, then the rates, then where; a laptop sets the last two side by side. -->
				<div class="flex-col gap-3.5 lg:grid lg:grid-cols-6 {on('climbs') ? 'flex' : 'hidden'}">
					<section class="flex flex-col gap-4 rounded-3xl bg-surface px-[18px] pt-4 pb-[18px] shadow-card lg:col-span-6">
						<div class="relative flex items-start gap-3">
							<dl class="grid flex-1 grid-cols-3 items-end gap-3">
								{@render stat('Climbs', climbCount(inPick(shown), data.unique))}
								{@render stat(`Climbs a ${data.per}`, average(climbs.map((c) => c.boulders + c.routes)))}
								{@render stat(data.unique ? 'Every go' : 'Once a session', climbCount(inPick(shown), !data.unique))}
							</dl>
							<Info text="Every go counts each try, sent or not, so three tries on one boulder are three climbs. Once a session, or Unique, counts each boulder or route once a session. {perInfo}" />
						</div>
						<!-- Boulders and routes each get a chart, on one scale, so each compares week to week. -->
						<!-- A laptop sets them side by side, so the card is as tall as the others. -->
						<div class="flex flex-col gap-4 lg:grid lg:grid-cols-2 lg:gap-9">
							{#each split.filter((k) => k.values.some((v) => v)) as k (k.label)}
								<div class="flex flex-col gap-1">
									<h4 class="text-[15px] font-semibold">{k.label}</h4>
									<Bars parts={[{ values: k.values, cls: 'bg-chart' }]} top={splitTop} height={84} {...charts} />
								</div>
							{/each}
						</div>
					</section>
					<!-- Send and flash rate answer one question, so they share a card and a scale. -->
					<section class="flex flex-col gap-4 rounded-3xl bg-surface px-[18px] pt-4 pb-[18px] shadow-card lg:col-span-3">
						<div class="relative flex items-start gap-3">
							<!-- Each label carries its line's colour, in place of a legend. -->
							<dl class="grid flex-1 grid-cols-3 items-end gap-3">
								<div class="flex flex-col gap-0.5">
									<dt class="flex items-center gap-1.5 text-xs font-semibold text-ink-2"><span class="w-3.5 border-t-2 border-chart"></span>Send rate</dt>
									<dd class="digits text-[26px] leading-tight">{pct(sendRate(inPick(shown)))}</dd>
								</div>
								<div class="flex flex-col gap-0.5">
									<dt class="flex items-center gap-1.5 text-xs font-semibold text-ink-2"><span class="w-3.5 border-t-2 border-dashed border-chart"></span>Flash rate</dt>
									<dd class="digits text-[26px] leading-tight">{pct(flashRate(inPick(shown)))}</dd>
								</div>
							</dl>
							<Info text="Of the graded climbs you tried, the share you sent (solid line), and the share you sent on the first try (dashed). Each climb counts once, however many tries it took. Limit work sends few; a rate near 100% means the climbs were easy." />
						</div>
						<Dots n={data.buckets.length} marks={line(sendRates)} also={line(flashRates)}	label={(x) => `${x}%`} zero max={100} {...charts} />
					</section>
					<section class="rounded-3xl bg-surface px-[18px] pt-4 pb-3 shadow-card lg:col-span-3">
						<div class="relative mb-2 flex items-start justify-between gap-3">
							<h3 class="text-[15px] font-bold">Where you climbed</h3>
							<Info text="Your climbs by where you climbed them, counted as the Climbs chart counts them." />
						</div>
						<ul class="flex flex-col gap-3">
							{#each mix as [where, n] (where)}
								<li class="flex flex-col gap-1.5">
									<span class="flex items-baseline justify-between gap-3 text-[15px] font-semibold">
										{where}
										<span class="digits">{n}</span>
									</span>
									<span class="h-2"
										><span class="block h-full rounded-full {where.endsWith('routes') ? routeFill : boulderFill}" style:width="{(n / mix[0][1]) * 100}%"></span></span
									>
								</li>
							{/each}
						</ul>
					</section>
				</div>
				<div class="flex-col gap-3.5 lg:mt-3 lg:flex {on('grade') ? 'flex' : 'hidden'}">
					<div class="lg:hidden">{@render sentTried()}</div>
					<div class="flex items-center gap-5 max-lg:hidden">
						<div class="flex items-baseline gap-3">
							{@render heading('Grades')}
							{#if filterQuery(data.filter)}<span class="text-[15px] font-semibold text-ink-2">{filterName(data.filter)}</span>{/if}
						</div>
						<div class="w-48 shrink-0">{@render sentTried()}</div>
					</div>
					<div class="flex flex-col gap-6 lg:grid lg:grid-cols-2 lg:gap-3.5">
						{#each disciplines as d (d.label)}
							<!-- The hardest send over time and the climbs at each grade answer
							     different questions, so each has its own card, under the discipline's
							     name. On a laptop the columns match, so the last cards end level. -->
							<div class="flex flex-col gap-2.5">
								<h3 class="px-1 text-[15px] font-bold">{d.label}</h3>
								<section class="flex flex-col gap-3 rounded-3xl bg-surface px-[18px] pt-4 pb-[18px] shadow-card">
									<div class="relative flex items-start justify-between gap-3">
										<div>
											<h4 class="text-xs font-semibold text-ink-2">Hardest send{#if when}{' · '}<span class="text-ink">{when}</span>{/if}</h4>
											<p class="digits text-[26px] leading-tight">{d.top}</p>
										</div>
										<Info text="Your hardest send each week, at its grade." />
									</div>
									<Dots n={data.buckets.length} marks={line(d.tops)} label={(g) => d.scale.grades[g]} steps limit={d.scale.grades.length - 1} {...charts} />
								</section>
								<section class="flex flex-1 flex-col gap-3 rounded-3xl bg-surface px-[18px] pt-4 pb-[18px] shadow-card">
									<div class="relative flex items-start justify-between gap-3">
										<div>
											<h4 class="text-xs font-semibold text-ink-2">By grade{#if when}{' · '}<span class="text-ink">{when}</span>{/if}</h4>
											<p class="digits text-[26px] leading-tight">
												{data.tried ? d.tried : d.sent}<span class="ml-1 text-xs font-semibold text-ink-2">{data.tried ? 'tried' : 'sent'}</span>
											</p>
										</div>
										<Info
											text={data.tried
												? 'Each climb you tried, once a session, at its grade. An outline is the ones you did not send that session. The middle of each bar is the ones you flashed.'
												: 'Each climb you sent, once, at its grade. The middle of each bar is the ones you flashed.'}
										/>
									</div>
									{#if d.pyramid.length}
										<div class="mt-1"><Pyramid rows={d.pyramid} most={d.most} all={data.tried} /></div>
										<div class="mt-auto flex flex-wrap gap-x-4 gap-y-1.5 text-xs font-semibold text-ink-2">
											{@render key('bg-chart', 'Flash')}
											{@render key('bg-chart/50', 'Sent after a try')}
											{#if data.tried}{@render key('ring-1 ring-inset ring-ink-3', 'Not sent')}{/if}
										</div>
									{/if}
									{#if d.elsewhere}
										<p class="text-xs font-semibold text-ink-2">
											{d.elsewhere}
											{d.elsewhere === 1 ? 'climb' : 'climbs'} graded in another system not shown.
										</p>
									{/if}
								</section>
							</div>
						{:else}
							<Empty title="No graded climbs" text="Give a climb its grade as you log it to see your grades here." />
						{/each}
					</div>
				</div>
			{/if}
		</div>

		<div class="flex-col gap-3.5 lg:mt-3 lg:flex {on('strength') ? 'flex' : 'hidden'}">
			{@render heading('Strength', 'max-lg:hidden')}
			{#if strength.length}
				<section class="rounded-3xl bg-surface px-[18px] pt-4 pb-2 shadow-card [--flush:-0.625rem]">
				<div class="relative flex items-start justify-between gap-3">
					<h3 class="text-[15px] font-bold">Best set a session</h3>
					<Info
						text="Each exercise in its own unit: its best set in a session, by added weight, else time under load, else reps. The figure is the latest session's; the change is from the range's first session to its latest."
					/>
				</div>
				<!-- Columns, not a grid, so a row without a line leaves no hole beside one with it.
				     Every row has its line on top, as a column cannot tell its first row. -->
				<ul class="lg:columns-2 lg:gap-x-9 lg:[&>li:first-child]:shadow-[inset_0_1px_0_var(--line)]">
					{#each strength as l (l.exercise)}
						{@const f = lift(l)}
						{@const marks = l.trend.map((v, i) => ({ i, v, best: true }))}
						<li class="list-row flush break-inside-avoid">
							<a href={liftHref(l)} class="flex flex-col gap-2.5 py-3.5">
								<span class="flex items-start gap-3">
									<span class="min-w-0 flex-1">
										<span class="block text-[15px] font-bold text-balance">{l.name}</span>
										<span class="mt-0.5 block text-xs font-semibold text-ink-2">{plural(l.sessions, 'session')}</span>
									</span>
									<span class="flex shrink-0 flex-col items-end">
										<span class="digits text-xl leading-tight">{f.main}<span class="ml-0.5 text-xs font-semibold text-ink-2">{f.small}</span></span>
										{#if f.note}<span class="mt-0.5 text-xs font-semibold text-ink-2">{f.note}</span>{/if}
									</span>
									<span class="self-center text-ink-3"><Icon name="chevron-right" size="16px" stroke={2.4} /></span>
								</span>
								{#if l.sessions > 1}
									<!-- Each lift's sessions from edge to edge, so every line starts and ends alike. -->
									<span class="block px-1.5"><Dots n={marks.length} {marks} label={(x) => `${x}`} zero={marks.some((m) => m.v < 0)} height={30} small spread /></span>
								{/if}
							</a>
						</li>
					{/each}
				</ul>
				</section>
			{:else}
				<Empty title="No sets" text="Log sets of a strength exercise in a session to see them here." />
			{/if}
		</div>
	</div>
{/if}

<Sheet bind:open={picking} title="Range">
	<div class="flex flex-col gap-3.5">
		<div class="grid grid-cols-3 gap-1 rounded-3xl bg-well p-1">
			{#each quick as q (q.label)}
				<button
					type="button"
					class="flex h-11 items-center justify-center rounded-full text-[15px] font-bold {data.span.key === q.key
						? 'segment-on'
						: 'text-ink-2'}"
					aria-pressed={data.span.key === q.key}
					onclick={() => pick(q.key)}>{q.label}</button
				>
			{/each}
		</div>
		{#if data.cycles.length}
			<h2 class="mt-1 px-1 text-[15px] font-bold">Cycles</h2>
			<ul class="rounded-3xl bg-surface px-[18px] py-2 shadow-card [--flush:-0.625rem]">
				{#each data.cycles as c (c.id)}
					{@const chosenCycle = data.span.key === `cycle:${c.id}`}
					<li class="list-row flush">
						<button type="button" class="flex min-h-[52px] w-full items-center gap-3 py-2 text-left" aria-pressed={chosenCycle} onclick={() => pick(`cycle:${c.id}`)}>
							<span class="min-w-0 flex-1 truncate text-[15px] font-bold">{c.name}</span>
							<span class="text-xs font-semibold text-ink-2">{shortDate(c.starts)} – {shortDate(c.ends)}</span>
							<span class="w-5 shrink-0 text-ink">{#if chosenCycle}<Icon name="check" size="1.25rem" stroke={2.6} />{/if}</span>
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

<Sheet bind:open={filtering} title="Climbs to show">
	<!-- Each group's chips name it, so the groups need no labels; a gap sets them apart. -->
	<div class="flex flex-col gap-6">
		{#each groups as g (g)}
			{@const closed = g === 'board' && !boardsOpen(draft)}
			<div class="flex flex-wrap gap-1.5" role="group" aria-label={filterGroups[g].label}>
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
