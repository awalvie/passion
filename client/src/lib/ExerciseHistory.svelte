<script lang="ts">
	import { formatDate, shortDate } from './dates';
	import Dots from './Dots.svelte';
	import Empty from './Empty.svelte';
	import type { Scale } from './grades';
	import Icon from './Icon.svelte';
	import { bestSet, climbMarks, clock, gradeScale, headline, measure, setLines, setMarks, topSend, climbTally, workText } from './progress';
	import type { HistorySession } from './run';
	import { plural } from './text';

	// An exercise's past sessions: the best effort and every set or climb as a
	// dot per session, then a row per session that lists its sets, or sums up
	// its climbs. Sides is 2 when each set is logged once a side; rows caps the
	// list, not the chart, and chart turns the chart off. With href, each row
	// links to where it says.
	let {
		sessions,
		scales = [],
		href,
		sides = 1,
		rows = Infinity,
		chart = true
	}: {
		sessions: HistorySession[];
		scales?: Scale[];
		href?: (s: HistorySession) => string;
		sides?: number;
		rows?: number;
		chart?: boolean;
	} = $props();

	const recent = $derived(sessions.slice(0, 12).reverse());
	const ticks = $derived(recent.map((s, i) => (i === 0 || i === recent.length - 1 ? shortDate(s.local_date) : '')));
	const scale = $derived(gradeScale(sessions.flatMap((s) => s.climbs), scales));
	const m = $derived(measure(sessions));
	const marks = $derived(recent.flatMap((s, i) => (scale ? climbMarks(s.climbs, i, scale) : setMarks(s.sets, i, m))));
	const head = $derived(headline(sessions, m, scale));
	const best = $derived(head && !scale ? bestSet(head.session.sets, m) : null);

	// A row names its session only when it is not the exercise's usual one,
	// and its date's year only when it is not this one.
	const usual = $derived.by(() => {
		const n = new Map<string, number>();
		for (const s of sessions) n.set(s.name, (n.get(s.name) ?? 0) + 1);
		return [...n].reduce<[string, number] | null>((a, b) => (!a || b[1] > a[1] ? b : a), null)?.[0];
	});
	const year = String(new Date().getFullYear());
	const day = (d: string) => formatDate(d, { weekday: 'short', day: 'numeric', month: 'short', ...(d.startsWith(year) ? {} : { year: 'numeric' }) });
	const said = (...parts: (string | false)[]) => parts.filter(Boolean).join(' · ');

	const label = (v: number) => (scale ? scale.grades[v] : m === 's' && v >= 60 ? clock(v) : `${v}`);
	const delta = (c: number, step: string) =>
		`${c > 0 ? '+' : '−'}${step === 'grade' ? plural(Math.abs(c), 'grade') : step === 's' && Math.abs(c) >= 60 ? clock(c) : `${Math.abs(c)} ${step}`}`;
</script>

<!-- A figure: its numbers in the number face, its words small and quiet. -->
{#snippet amount(text: string, size = 'text-[15px]')}
	<span class="text-xs font-semibold text-ink-2">
		{#each text.split(/([\d.:/]+)/) as part, i (i)}
			{#if i % 2}<span class="digits {size} font-semibold text-ink">{part}</span>{:else}{part}{/if}
		{/each}
	</span>
{/snippet}

{#snippet chevron()}
	{#if href}<span class="self-center text-ink-3 lg:hidden"><Icon name="chevron-right" size="16px" stroke={2.6} /></span>{/if}
{/snippet}

<div class="flex flex-col gap-3">
	{#if chart && head && recent.length > 1}
		<section class="flex flex-col gap-4 rounded-3xl bg-surface p-[18px] shadow-card-sm">
			<div class="flex items-end justify-between gap-3">
				<div>
					<p class="text-xs font-semibold text-ink-2">{scale ? 'Hardest send' : 'Top set'} last time · {shortDate(head.session.local_date)}</p>
					{#if best}
						<!-- Weight, then reps, as each set below reads: "3 × 80" read as three sets of 80. -->
						<p class="mt-1 digits text-[32px] leading-none">
							{#if best.weight_kg}{best.weight_kg}<span class="ml-1 text-[15px] text-ink-2">kg · {workText(best)}</span>{:else}{@render amount(workText(best), 'text-[32px]')}{/if}
						</p>
					{:else}
						<p class="mt-1 digits text-[32px] leading-none">{head.main}{#if head.small}<span class="ml-1 text-[15px] text-ink-2">{head.small}</span>{/if}</p>
					{/if}
				</div>
				{#if head.change}
					<span class="rounded-full px-2.5 py-1 digits text-xs {head.change > 0 ? 'bg-chart/15 text-ink' : 'bg-well text-ink-2'}"
						>{delta(head.change, head.step)}</span
					>
				{/if}
			</div>
			<Dots n={recent.length} {marks} {label} zero={!scale} height={110} {ticks} />
		</section>
	{/if}

	{#if sessions.length}
		<section class="rounded-3xl bg-surface px-[18px] py-1 shadow-card-sm">
			<ul>
				{#each sessions.slice(0, rows) as session (session.run)}
					<li class="list-row">
						{#if scale || (session.climbs.length && !session.sets.length)}
							{@const send = scale ? topSend(session.climbs, scale) : null}
							<svelte:element this={href ? 'a' : 'div'} href={href?.(session)} class="flex items-center gap-3.5 py-3">
								<span class="flex size-11 shrink-0 flex-col items-center justify-center rounded-xl bg-well">
									<span class="text-xs leading-none font-bold text-ink-2 uppercase">{formatDate(session.local_date, { month: 'short' })}</span>
									<span class="mt-0.5 digits text-lg leading-none">{Number(session.local_date.slice(8))}</span>
								</span>
								<span class="min-w-0 flex-1">
									{#if send}<span class="block digits text-xl leading-tight">{send.grade}</span>{/if}
									<span class="mt-0.5 line-clamp-2 text-xs font-semibold text-ink-2">
										{said(!session.local_date.startsWith(year) && session.local_date.slice(0, 4), session.name !== usual && session.name, climbTally(session.climbs))}{scale ? `, ${session.climbs.filter((c) => c.sent).length} sent` : ''}
									</span>
								</span>
								{@render chevron()}
							</svelte:element>
						{:else}
							<!-- Every set in order, as History's session view numbers them, alike sets on one line. -->
							{@const lines = setLines(session.sets, sides, m === 'kg')}
							{@const note = said(session.name !== usual && session.name, sides === 2 && lines.length > 0 && 'each side')}
							<svelte:element this={href ? 'a' : 'div'} href={href?.(session)} class="flex items-start gap-3.5 py-3">
								<span class="min-w-0 flex-1">
									<span class="flex min-w-0 items-baseline gap-2">
										<span class="shrink-0 text-[15px] font-bold">{day(session.local_date)}</span>
										{#if note}<span class="min-w-0 truncate text-xs font-semibold text-ink-2">{note}</span>{/if}
									</span>
									{#if lines.length}
										<span class="mt-1.5 grid w-fit max-w-full grid-cols-[2.25rem_minmax(0,6rem)_minmax(4.5rem,max-content)] items-baseline gap-x-2 gap-y-1">
											{#each lines as l (l.from)}
												<span class="digits text-xs font-semibold text-ink-2">{l.from === l.to ? l.from : `${l.from}–${l.to}`}</span>
												<span class="truncate">{@render amount(l.work)}</span>
												<span class="text-right">{@render amount(l.load)}</span>
											{/each}
										</span>
									{:else}
										<span class="mt-1 block text-xs font-semibold text-ink-2">No sets finished</span>
									{/if}
								</span>
								{@render chevron()}
							</svelte:element>
						{/if}
					</li>
				{/each}
			</ul>
		</section>
	{:else}
		<Empty title="Nothing logged yet" text="Log this exercise in a session to see it here." />
	{/if}
</div>
