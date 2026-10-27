<script lang="ts">
	import { page } from '$app/state';
	import { MediaQuery } from 'svelte/reactivity';
	import { describe, request } from '$lib/api';
	import Bars from '$lib/Bars.svelte';
	import { formatDate, weekday } from '$lib/dates';
	import { formatMinutes } from '$lib/exercise';
	import Dots from '$lib/Dots.svelte';
	import FormError from '$lib/FormError.svelte';
	import Icon from '$lib/Icon.svelte';
	import { counts, gradeScale, grades, hours, isBoulder } from '$lib/progress';
	import type { Run, RunSummary } from '$lib/run';
	import RunView from '$lib/RunView.svelte';

	let { data } = $props();

	// From lg a session opens in a pane beside the list, named by open; with
	// none named, the newest opens.
	const wide = new MediaQuery('min-width: 64rem');
	const openId = $derived(wide.current ? page.url.searchParams.get('open') || data.runs[0]?.id : undefined);
	let opened = $state<Run | null>(null);
	let paneError = $state('');
	$effect(() => {
		const id = openId;
		if (!id) return;
		let gone = false;
		paneError = '';
		request<Run>('GET', `/api/v1/runs/${id}`).then(
			(r) => !gone && (opened = r),
			(e) => !gone && (paneError = describe(e, (f) => f))
		);
		return () => (gone = true);
	});

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
		const length = r.elapsed_seconds === null ? '' : formatMinutes(r.elapsed_seconds);
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
	const hoursLabel = (h: number) => `${Math.round(h * 10) / 10}h`;
</script>

<svelte:head><title>History</title></svelte:head>

{#snippet tile(label: string, value: string, note: string)}
	<span class="flex items-baseline justify-between gap-1">
		<span class="text-xs font-bold text-ink-2">{label}</span>
		<!-- Beside the list a tile has no room for "Hardest send" and a chevron. -->
		<span class="text-ink-3 lg:hidden"><Icon name="chevron-right" size="14px" stroke={2.6} /></span>
	</span>
	<span class="mt-1 digits text-[22px] leading-none">{value}</span>
	<span class="mt-0.5 text-[11px] font-semibold text-ink-2">{note}</span>
{/snippet}

<div class="lg:mx-auto lg:grid lg:max-w-[1280px] lg:grid-cols-[420px_minmax(0,1fr)]">
	<div class="flex flex-col gap-3.5 px-4 pt-[calc(env(safe-area-inset-top)+0.75rem)] pb-6 lg:border-r lg:border-line lg:px-6 lg:pt-5">
		<h1 class="page-title">History</h1>

		{#if data.runs.length}
			<nav class="grid grid-cols-3 gap-2.5" aria-label="Progress">
				<a href="/history/progress?view=time" class="flex min-w-0 flex-col rounded-3xl bg-surface px-3.5 pt-3 pb-3.5 shadow-card">
					{@render tile('Time', hoursLabel(time.at(-1)!), 'this week')}
					<span class="mt-3.5"><Bars parts={[{ values: time, cls: 'bg-chart' }]} height={30} small /></span>
				</a>
				<a href="/history/progress?view=climbs" class="flex min-w-0 flex-col rounded-3xl bg-surface px-3.5 pt-3 pb-3.5 shadow-card">
					{@render tile('Climbs', String(climbs.at(-1)!.boulders + climbs.at(-1)!.routes), 'this week')}
					<span class="mt-3.5">
						<Bars
							parts={[
								{ values: climbs.map((c) => c.boulders), cls: 'bg-chart' },
								{ values: climbs.map((c) => c.routes), cls: 'bg-chart/50' }
							]}
							height={30}
							small
						/>
					</span>
				</a>
				<a href="/history/progress?view=grade" class="flex min-w-0 flex-col rounded-3xl bg-surface px-3.5 pt-3 pb-3.5 shadow-card">
					{@render tile('Hardest send', top ?? '–', '10 weeks')}
					<span class="mt-3.5">
						{#if main}<Dots
								n={main.points.length}
								marks={main.points.flatMap((v, i) => (v === null ? [] : [{ i, v, best: true }]))}
								label={(g) => main.scale.grades[g]}
								height={30}
								small
							/>{:else}<span class="block h-[30px]"></span>{/if}
					</span>
				</a>
			</nav>
		{/if}

		{#each months as m (m.label)}
			<section class="rounded-3xl bg-surface px-[18px] pt-4 pb-2 shadow-card">
				<h2 class="mb-1 text-[15px] font-bold">{m.label}</h2>
				<ul>
					{#each m.runs as r (r.id)}
						{@const picked = r.id === openId}
						<li class="list-row {picked ? 'picked' : ''}">
							<a
								href={wide.current ? `/history?open=${r.id}` : `/history/${r.id}`}
								data-sveltekit-replacestate={wide.current ? '' : undefined}
								data-sveltekit-noscroll={wide.current ? '' : undefined}
								aria-current={picked ? 'true' : undefined}
								class="flex min-h-16 items-center gap-3.5 py-2.5"
							>
								<span class="flex size-12 shrink-0 flex-col items-center justify-center rounded-2xl {picked ? 'bg-surface' : 'bg-well'}">
									<span class="text-[10px] leading-none font-bold tracking-wide text-ink-2 uppercase">{weekday(r.local_date)}</span>
									<span class="mt-0.5 digits text-xl leading-none">{Number(r.local_date.slice(8))}</span>
								</span>
								<span class="min-w-0 flex-1">
									<span class="block truncate text-[15px] font-bold">{r.name}</span>
									{#if facts(r)}<span class="mt-0.5 block truncate text-xs font-semibold text-ink-2">{facts(r)}</span>{/if}
								</span>
								<span class="text-ink-3 lg:hidden"><Icon name="chevron-right" size="18px" stroke={2.4} /></span>
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

	<!-- The pane scrolls on its own, so a long session leaves the list where it was. -->
	<div class="hidden lg:sticky lg:top-0 lg:block lg:h-dvh lg:overflow-y-auto">
		<div class="mx-auto flex max-w-[640px] flex-col gap-3.5 px-8 pt-3 pb-8">
			<FormError message={paneError} />
			{#if opened}
				{@const run = opened}
				<div class="flex justify-end gap-2">
					<a href="/run/{run.id}" class="flex h-11 items-center rounded-full bg-surface px-4 text-[15px] font-bold text-ink shadow-card-sm">Edit</a>
				</div>
				<RunView {run} heading="h2" exerciseHref={(s) => `/history/exercise/${s.exercise}?from=/history/${run.id}&name=${encodeURIComponent(s.name)}`} />
			{/if}
		</div>
	</div>
</div>

<style>
	/* The open session's row, drawn as a well that reaches past the card's
	   dividers, as Journal's. */
	.picked {
		margin-inline: -0.5rem;
		padding-inline: 0.5rem;
		border-radius: 1rem;
		background: var(--well);
		box-shadow: none;
	}
	.picked + :global(li) {
		box-shadow: none;
	}
</style>
