<script lang="ts">
	import { goto } from '$app/navigation';
	import Bars from '$lib/Bars.svelte';
	import Button from '$lib/Button.svelte';
	import { shortDate } from '$lib/dates';
	import DateField from '$lib/DateField.svelte';
	import Dots from '$lib/Dots.svelte';
	import Icon from '$lib/Icon.svelte';
	import NavBar from '$lib/NavBar.svelte';
	import {
		boardsOpen,
		counts,
		filterGroups,
		filterName,
		filterQuery,
		flashRate,
		goesPerSend,
		gradeScale,
		grades,
		hours,
		isBoulder,
		mixOf,
		monthsBack,
		passes,
		perBucket,
		sendRate,
		type ClimbFilter,
		type FilterGroup
	} from '$lib/progress';
	import Segmented from '$lib/Segmented.svelte';
	import Sheet from '$lib/Sheet.svelte';

	let { data } = $props();

	const href = (view: string, range: string, filter = data.filter) => `?view=${view}&range=${range}${filterQuery(filter)}`;
	const round = (h: number) => Math.round(h * 10) / 10;
	const pct = (v: number | null) => (v === null ? '–' : `${v}%`);
	const ticks = $derived(data.buckets.map((b) => b.tick));
	// A week still to come counts for nothing in an average.
	const passed = $derived(data.now === -1 ? data.buckets.length : data.now + 1);
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
		for (const r of data.runs) {
			const e = m.get(r.name) ?? { n: 0, h: 0 };
			m.set(r.name, { n: e.n + 1, h: e.h + (r.elapsed_seconds ?? 0) / 3600 });
		}
		return [...m].sort((a, b) => b[1].h - a[1].h);
	});

	const shown = $derived(data.climbs.filter((c) => passes(c, data.filter)));
	const climbs = $derived(counts(shown, data.buckets));
	const sent = $derived(shown.filter((c) => c.sent).length);
	const sendRates = $derived(perBucket(shown, data.buckets, sendRate));
	const flashRates = $derived(perBucket(shown, data.buckets, flashRate));
	// Where the climbing was, most first.
	const mix = $derived.by(() => {
		const m = new Map<string, number>();
		for (const c of shown) m.set(mixOf(c), (m.get(mixOf(c)) ?? 0) + 1);
		return [...m].sort((a, b) => b[1] - a[1]);
	});

	// Boulders and routes apart, as their grades are on different scales.
	const disciplines = $derived(
		[
			{ label: 'Boulders', climbs: shown.filter(isBoulder) },
			{ label: 'Routes', climbs: shown.filter((c) => !isBoulder(c)) }
		].flatMap((d) => {
			const scale = gradeScale(d.climbs, data.scales);
			if (!scale) return [];
			const points = grades(d.climbs, data.buckets, scale);
			const tops = points.filter((t): t is number => t !== null);
			return [{ ...d, scale, points, top: tops.length ? scale.grades[Math.max(...tops)] : null }];
		})
	);

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

	function applyFilter(f: ClimbFilter) {
		filtering = false;
		goto(href(data.view, data.span.key, { ...f, board: boardsOpen(f) ? f.board : [] }), { replaceState: true, noScroll: true });
	}
</script>

<svelte:head><title>Progress</title></svelte:head>

{#snippet stat(label: string, value: string | number)}
	<div class="flex flex-col gap-0.5">
		<dt class="text-xs font-semibold text-ink-2">{label}</dt>
		<dd class="digits text-[26px] leading-tight">{value}</dd>
	</div>
{/snippet}

{#snippet key(cls: string, label: string)}
	<span class="flex items-center gap-1.5"><span class="size-2.5 rounded-full {cls}"></span>{label}</span>
{/snippet}

{#snippet head(title: string, figure?: string, small?: string)}
	<div class="flex items-baseline justify-between gap-3">
		<h2 class="text-[15px] font-bold">{title}</h2>
		{#if figure}<span class="text-xs font-semibold text-ink-2">{small} <span class="digits text-[17px] text-ink">{figure}</span></span>{/if}
	</div>
{/snippet}

<NavBar back={{ href: '/history', label: 'History' }} title="Progress" />

<div class="flex flex-col gap-3.5 px-4 pt-2 pb-6">
	<Segmented
		label="What to show"
		replace
		items={[
			{ label: 'Time', on: data.view === 'time', href: href('time', data.span.key) },
			{ label: 'Climbs', on: data.view === 'climbs', href: href('climbs', data.span.key) },
			{ label: 'Grade', on: data.view === 'grade', href: href('grade', data.span.key) }
		]}
	/>
	<nav class="flex gap-1.5" aria-label="Range">
		{#each suggested as s (s.key)}
			<a
				href={href(data.view, s.key)}
				data-sveltekit-replacestate
				data-sveltekit-noscroll
				aria-current={data.span.key === s.key ? 'true' : undefined}
				class="flex h-11 shrink-0 items-center rounded-full px-4 text-[15px] font-bold whitespace-nowrap {data.span.key === s.key
					? 'bg-ink text-ground'
					: 'bg-surface text-ink shadow-card-sm'}"
			>
				{s.label}
			</a>
		{/each}
		<button
			type="button"
			class="flex h-11 min-w-0 flex-1 items-center justify-center gap-1.5 rounded-full px-4 text-[15px] font-bold {custom
				? 'bg-ink text-ground'
				: 'bg-surface text-ink shadow-card-sm'}"
			aria-current={custom ? 'true' : undefined}
			aria-haspopup="dialog"
			onclick={openPicker}
		>
			<Icon name="calendar" size="1rem" />
			<span class="truncate">Custom</span>
		</button>
	</nav>
	<div class="mt-1 flex flex-col px-1">
		{#if data.view === 'time'}
			<h2 class="text-xl font-bold tracking-tight">All sessions</h2>
		{:else}
			<button type="button" class="-my-2 flex min-h-11 items-center gap-1 self-start text-left text-xl font-bold tracking-tight" aria-haspopup="dialog" onclick={openFilter}>
				{filterName(data.filter)}
				<Icon name="chevron-down" size="1.125rem" stroke={2.6} />
			</button>
		{/if}
		<p class="text-xs font-semibold text-ink-2">
			{data.span.cycle && custom ? `${data.span.cycle.name} · ` : ''}{shortDate(data.span.from)} – {shortDate(data.span.to)}
		</p>
	</div>

	{#if data.view === 'time'}
		<section class="flex flex-col gap-4 rounded-3xl bg-surface px-[18px] pt-4 pb-[18px] shadow-card">
			<dl class="grid grid-cols-3 gap-3">
				{@render stat('Hours', round(totalHours))}
				{@render stat(`A ${data.per}`, round(totalHours / passed))}
				{@render stat('Sessions', data.runs.length)}
			</dl>
			<Bars parts={[{ values: time, cls: 'bg-chart' }]} format={(h) => `${round(h)}h`} {ticks} now={data.now} />
		</section>
		{#if bySession.length}
			<section class="rounded-3xl bg-surface px-[18px] pt-4 pb-2 shadow-card">
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
	{:else if !shown.length}
		<section class="rounded-3xl bg-surface p-5 shadow-card">
			<p class="text-[15px] font-semibold text-ink-2">{filterQuery(data.filter) ? 'No climbs match in this range.' : 'No climbs in this range.'}</p>
		</section>
	{:else if data.view === 'climbs'}
		<section class="flex flex-col gap-4 rounded-3xl bg-surface px-[18px] pt-4 pb-[18px] shadow-card">
			<dl class="grid grid-cols-3 gap-3">
				{@render stat('Climbs', shown.length)}
				{@render stat(`A ${data.per}`, round(shown.length / passed))}
				{@render stat('Sent', sent)}
			</dl>
			<Bars
				parts={[
					{ values: climbs.map((c) => c.boulders), cls: boulderFill },
					{ values: climbs.map((c) => c.routes), cls: routeFill }
				]}
				{ticks}
				now={data.now}
			/>
			<div class="flex gap-4 text-xs font-semibold text-ink-2">
				{#if shown.some(isBoulder)}{@render key(boulderFill, 'Boulders')}{/if}
				{#if shown.some((c) => !isBoulder(c))}{@render key(routeFill, 'Routes')}{/if}
			</div>
		</section>
		<section class="flex flex-col gap-4 rounded-3xl bg-surface px-[18px] pt-4 pb-[18px] shadow-card">
			{@render head('Send rate', pct(sendRate(shown)), 'Overall')}
			<Dots n={data.buckets.length} marks={line(sendRates)} label={(v) => `${v}%`} zero {ticks} now={data.now} />
		</section>
		<section class="flex flex-col gap-4 rounded-3xl bg-surface px-[18px] pt-4 pb-[18px] shadow-card">
			<h2 class="text-[15px] font-bold">First go</h2>
			<dl class="grid grid-cols-3 gap-3">
				{@render stat('Flash rate', pct(flashRate(shown)))}
				{@render stat('Goes a send', goesPerSend(shown) ?? '–')}
			</dl>
			<Dots n={data.buckets.length} marks={line(flashRates)} label={(v) => `${v}%`} zero {ticks} now={data.now} />
			<p class="-mt-2 text-xs font-semibold text-ink-2">Flash rate each {data.per}</p>
		</section>
		<section class="rounded-3xl bg-surface px-[18px] pt-4 pb-3 shadow-card">
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
	{:else}
		{#each disciplines as d (d.label)}
			<section class="flex flex-col gap-4 rounded-3xl bg-surface px-[18px] pt-4 pb-[18px] shadow-card">
				{@render head(d.label, d.top ?? undefined, 'Hardest send')}
				<Dots n={d.points.length} marks={line(d.points)} label={(g) => d.scale.grades[g]} {ticks} now={data.now} />
			</section>
		{:else}
			<section class="rounded-3xl bg-surface p-5 shadow-card">
				<p class="text-[15px] font-semibold text-ink-2">No graded climbs in this range.</p>
			</section>
		{/each}
	{/if}
</div>

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
					<DateField bind:value={from} label="From" max={data.today} />
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
