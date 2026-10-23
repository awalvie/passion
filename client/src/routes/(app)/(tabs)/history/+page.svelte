<script lang="ts">
	import Bars from '$lib/Bars.svelte';
	import { formatDate, weekday } from '$lib/dates';
	import { formatDuration } from '$lib/exercise';
	import GradeDots from '$lib/GradeDots.svelte';
	import Icon from '$lib/Icon.svelte';
	import { counts, gradeScale, grades, hours, isBoulder } from '$lib/progress';
	import type { RunSummary } from '$lib/run';

	let { data } = $props();

	const months = $derived.by(() => {
		const out: { label: string; runs: RunSummary[] }[] = [];
		for (const r of data.runs) {
			const label = formatDate(r.local_date, { month: 'long', year: 'numeric' });
			if (out.at(-1)?.label !== label) out.push({ label, runs: [] });
			out.at(-1)!.runs.push(r);
		}
		return out;
	});

	function facts(r: RunSummary) {
		const length = r.elapsed_seconds === null ? '' : formatDuration(Math.max(1, Math.round(r.elapsed_seconds / 60)) * 60);
		return [length, r.place].filter(Boolean).join(' · ');
	}

	const time = $derived(hours(data.runs, data.weeks));
	const climbs = $derived(counts(data.climbs, data.weeks));
	// The thumbnail follows whichever of boulders and routes was climbed more.
	const main = $derived.by(() => {
		const boulders = data.climbs.filter(isBoulder);
		const mine = boulders.length * 2 >= data.climbs.length ? boulders : data.climbs.filter((c) => !isBoulder(c));
		const scale = gradeScale(mine, data.scales);
		return scale && { scale, points: grades(mine, data.weeks, scale) };
	});
	const top = $derived.by(() => {
		const tops = main?.points.filter((t): t is number => t !== null) ?? [];
		return tops.length ? main!.scale.grades[Math.max(...tops)] : null;
	});
	const hoursLabel = (h: number) => (h ? `${Math.round(h * 2) / 2}h` : '0h');
</script>

<svelte:head><title>History</title></svelte:head>

{#snippet tile(label: string, value: string, note: string)}
	<span class="flex items-baseline justify-between gap-1">
		<span class="text-xs font-bold text-ink-2">{label}</span>
		<span class="text-ink-3"><Icon name="chevron-right" size="14px" stroke={2.6} /></span>
	</span>
	<span class="mt-1 digits text-[22px] leading-none">{value}</span>
	<span class="mt-0.5 text-[11px] font-semibold text-ink-2">{note}</span>
{/snippet}

<div class="flex flex-col gap-3.5 px-4 pt-[calc(env(safe-area-inset-top)+0.75rem)] pb-6">
	<h1 class="page-title">History</h1>

	{#if data.runs.length}
		<nav class="grid grid-cols-3 gap-2.5" aria-label="Progress">
			<a href="/history/progress?view=time" class="flex min-w-0 flex-col rounded-3xl bg-surface px-3.5 pt-3 pb-3.5 shadow-card">
				{@render tile('Time', hoursLabel(time.at(-1)!), 'this week')}
				<span class="mt-3.5"><Bars parts={[{ values: time, cls: 'bg-rest-on-hero' }]} height={30} small /></span>
			</a>
			<a href="/history/progress?view=climbs" class="flex min-w-0 flex-col rounded-3xl bg-surface px-3.5 pt-3 pb-3.5 shadow-card">
				{@render tile('Climbs', String(climbs.at(-1)!.boulders + climbs.at(-1)!.routes), 'this week')}
				<span class="mt-3.5">
					<Bars
						parts={[
							{ values: climbs.map((c) => c.boulders), cls: 'bg-rest-on-hero' },
							{ values: climbs.map((c) => c.routes), cls: 'bg-[color-mix(in_oklab,var(--rest-on-hero),var(--hero-2))]' }
						]}
						height={30}
						small
					/>
				</span>
			</a>
			<a href="/history/progress?view=grade" class="flex min-w-0 flex-col rounded-3xl bg-surface px-3.5 pt-3 pb-3.5 shadow-card">
				{@render tile('Top send', top ?? '–', '10 weeks')}
				<span class="mt-3.5">
					{#if main}<GradeDots points={main.points} grades={main.scale.grades} height={30} small />{:else}<span class="block h-[30px]"></span>{/if}
				</span>
			</a>
		</nav>
	{/if}

	{#each months as m (m.label)}
		<section class="rounded-3xl bg-surface px-[18px] pt-4 pb-2 shadow-card">
			<h2 class="mb-1 text-[15px] font-bold">{m.label}</h2>
			<ul>
				{#each m.runs as r (r.id)}
					<li class="list-row">
						<a href="/history/{r.id}" class="flex min-h-16 items-center gap-3.5 py-2.5">
							<span class="flex size-12 shrink-0 flex-col items-center justify-center rounded-2xl bg-well">
								<span class="text-[10px] leading-none font-bold tracking-wide text-ink-2 uppercase">{weekday(r.local_date)}</span>
								<span class="mt-0.5 digits text-xl leading-none">{Number(r.local_date.slice(8))}</span>
							</span>
							<span class="min-w-0 flex-1">
								<span class="block truncate text-[15px] font-bold">{r.name}</span>
								{#if facts(r)}<span class="mt-0.5 block truncate text-xs font-semibold text-ink-2">{facts(r)}</span>{/if}
							</span>
							<span class="text-ink-3"><Icon name="chevron-right" size="18px" stroke={2.4} /></span>
						</a>
					</li>
				{/each}
			</ul>
		</section>
	{:else}
		<section class="flex flex-col gap-3 rounded-3xl bg-surface p-5 shadow-card">
			<p class="text-xl font-bold tracking-tight">No sessions yet</p>
			<p class="text-[15px] font-semibold text-ink-2">Finished sessions show here.</p>
			<a href="/" class="flex h-12 items-center justify-center rounded-full bg-tint text-[15px] font-bold text-on-tint shadow-tint">Start one</a>
		</section>
	{/each}
</div>
