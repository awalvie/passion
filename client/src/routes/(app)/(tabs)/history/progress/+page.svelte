<script lang="ts">
	import { goto } from '$app/navigation';
	import Bars from '$lib/Bars.svelte';
	import Button from '$lib/Button.svelte';
	import { shortDate } from '$lib/dates';
	import DateField from '$lib/DateField.svelte';
	import Dots from '$lib/Dots.svelte';
	import Icon from '$lib/Icon.svelte';
	import NavBar from '$lib/NavBar.svelte';
	import { counts, gradeScale, grades, hours, isBoulder, monthsBack } from '$lib/progress';
	import Segmented from '$lib/Segmented.svelte';
	import Sheet from '$lib/Sheet.svelte';

	let { data } = $props();

	const href = (view: string, range: string) => `?view=${view}&range=${range}`;
	const round = (h: number) => Math.round(h * 10) / 10;
	const ticks = $derived(data.buckets.map((b) => b.tick));
	// A week still to come counts for nothing in an average.
	const passed = $derived(data.now === -1 ? data.buckets.length : data.now + 1);
	const boulderFill = 'bg-chart';
	const routeFill = 'bg-chart/50';

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

	const climbs = $derived(counts(data.climbs, data.buckets));
	const sent = $derived(data.climbs.filter((c) => c.sent).length);

	const disciplines = $derived(
		[
			{ label: 'Boulders', climbs: data.climbs.filter(isBoulder) },
			{ label: 'Routes', climbs: data.climbs.filter((c) => !isBoulder(c)) }
		].flatMap((d) => {
			const scale = gradeScale(d.climbs, data.scales);
			if (!scale) return [];
			const points = grades(d.climbs, data.buckets, scale);
			const tops = points.filter((t): t is number => t !== null);
			return [{ ...d, scale, points, top: tops.length ? scale.grades[Math.max(...tops)] : null }];
		})
	);
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
	<p class="-mt-1.5 px-1 text-xs font-semibold text-ink-2">
		{data.span.cycle && custom ? `${data.span.cycle.name} · ` : ''}{shortDate(data.span.from)} – {shortDate(data.span.to)}
	</p>

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
	{:else if data.view === 'climbs'}
		<section class="flex flex-col gap-4 rounded-3xl bg-surface px-[18px] pt-4 pb-[18px] shadow-card">
			<dl class="grid grid-cols-3 gap-3">
				{@render stat('Climbs', data.climbs.length)}
				{@render stat(`A ${data.per}`, round(data.climbs.length / passed))}
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
				{@render key(boulderFill, 'Boulders')}
				{@render key(routeFill, 'Routes')}
			</div>
		</section>
	{:else}
		{#each disciplines as d (d.label)}
			<section class="flex flex-col gap-4 rounded-3xl bg-surface px-[18px] pt-4 pb-[18px] shadow-card">
				<div class="flex items-baseline justify-between gap-3">
					<h2 class="text-[15px] font-bold">{d.label}</h2>
					{#if d.top}<span class="text-xs font-semibold text-ink-2">Hardest send <span class="digits text-[17px] text-ink">{d.top}</span></span>{/if}
				</div>
				<Dots
					n={d.points.length}
					marks={d.points.flatMap((v, i) => (v === null ? [] : [{ i, v, best: true }]))}
					label={(g) => d.scale.grades[g]}
					{ticks}
					now={data.now}
				/>
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
