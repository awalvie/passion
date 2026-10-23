<script lang="ts">
	import Bars from '$lib/Bars.svelte';
	import GradeDots from '$lib/GradeDots.svelte';
	import NavBar from '$lib/NavBar.svelte';
	import { counts, gradeScale, grades, hours, isBoulder, ranges } from '$lib/progress';
	import Segmented from '$lib/Segmented.svelte';

	let { data } = $props();

	const per = $derived(data.range === '1y' ? 'month' : 'week');
	const href = (view: string, range: string) => `?view=${view}&range=${range}`;
	const round = (h: number) => Math.round(h * 10) / 10;
	const ticks = $derived(data.buckets.map((b) => b.tick));
	const boulderFill = 'bg-rest-on-hero';
	const routeFill = 'bg-[color-mix(in_oklab,var(--rest-on-hero),var(--hero-2))]';

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
			{ label: 'Time', on: data.view === 'time', href: href('time', data.range) },
			{ label: 'Climbs', on: data.view === 'climbs', href: href('climbs', data.range) },
			{ label: 'Grade', on: data.view === 'grade', href: href('grade', data.range) }
		]}
	/>
	<nav class="flex justify-center gap-1.5" aria-label="Range">
		{#each ranges as r (r.range)}
			<a
				href={href(data.view, r.range)}
				data-sveltekit-replacestate
				data-sveltekit-noscroll
				aria-current={data.range === r.range ? 'page' : undefined}
				class="flex h-9 items-center rounded-full px-3.5 text-[13px] font-bold {data.range === r.range ? 'bg-ink text-ground' : 'text-ink-2'}"
			>
				{r.label}
			</a>
		{/each}
	</nav>

	{#if data.view === 'time'}
		<section class="flex flex-col gap-4 rounded-3xl bg-surface px-[18px] pt-4 pb-[18px] shadow-card">
			<dl class="grid grid-cols-3 gap-3">
				{@render stat('Hours', round(totalHours))}
				{@render stat(`A ${per}`, round(totalHours / data.buckets.length))}
				{@render stat('Sessions', data.runs.length)}
			</dl>
			<Bars parts={[{ values: time, cls: 'bg-rest-on-hero' }]} format={(h) => `${round(h)}h`} {ticks} />
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
				{@render stat(`A ${per}`, round(data.climbs.length / data.buckets.length))}
				{@render stat('Sent', sent)}
			</dl>
			<Bars
				parts={[
					{ values: climbs.map((c) => c.boulders), cls: boulderFill },
					{ values: climbs.map((c) => c.routes), cls: routeFill }
				]}
				{ticks}
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
				<GradeDots points={d.points} grades={d.scale.grades} {ticks} />
			</section>
		{:else}
			<section class="rounded-3xl bg-surface p-5 shadow-card">
				<p class="text-[15px] font-semibold text-ink-2">No graded climbs in this range.</p>
			</section>
		{/each}
	{/if}
</div>
