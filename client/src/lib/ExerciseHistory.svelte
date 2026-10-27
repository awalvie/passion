<script lang="ts">
	import { formatDate, shortDate } from './dates';
	import Dots from './Dots.svelte';
	import type { Scale } from './grades';
	import { bestSet, climbMarks, clock, figure, gradeScale, headline, measure, setMarks, topSend } from './progress';
	import type { HistorySession } from './run';
	import { plural } from './text';

	// An exercise's past sessions: the best effort and every set or climb as a
	// dot per session, then one quiet row per session. Sides is 2 when each set
	// is logged once a side; rows caps the list, not the chart, and chart turns
	// the chart off. With href, each row links to where it says.
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

	const label = (v: number) => (scale ? scale.grades[v] : m === 's' && v >= 60 ? clock(v) : `${v}`);
	const delta = (c: number, step: string) =>
		`${c > 0 ? '+' : '−'}${step === 'grade' ? plural(Math.abs(c), 'grade') : step === 's' && Math.abs(c) >= 60 ? clock(c) : `${Math.abs(c)} ${step}`}`;
</script>

<div class="flex flex-col gap-3">
	{#if chart && head && recent.length > 1}
		<section class="flex flex-col gap-4 rounded-3xl bg-surface p-4 shadow-card-sm">
			<div class="flex items-end justify-between gap-3">
				<div>
					<p class="text-xs font-semibold text-ink-2">{scale ? 'Hardest send' : 'Top set'} last time · {shortDate(head.session.local_date)}</p>
					<p class="mt-1 digits text-[32px] leading-none">{head.main}{#if head.small}<span class="ml-1 text-[15px] text-ink-2">{head.small}</span>{/if}</p>
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
		<section class="rounded-3xl bg-surface px-4 py-1 shadow-card-sm">
			<ul>
				{#each sessions.slice(0, rows) as session (session.run)}
					{@const fig = scale ? null : bestSet(session.sets, m)}
					{@const send = scale ? topSend(session.climbs, scale) : null}
					{@const [main, unit] = fig ? figure(fig, m, true) : ['', '']}
					<li class="list-row">
						<svelte:element this={href ? 'a' : 'div'} href={href?.(session)} class="flex items-center gap-3.5 py-3">
							<span class="flex size-11 shrink-0 flex-col items-center justify-center rounded-xl bg-well">
								<span class="text-xs leading-none font-bold text-ink-2 uppercase">{formatDate(session.local_date, { month: 'short' })}</span>
								<span class="mt-0.5 digits text-lg leading-none">{Number(session.local_date.slice(8))}</span>
							</span>
							<span class="min-w-0 flex-1">
								{#if fig}
									<span class="block digits text-xl leading-tight">{main}<span class="ml-1 text-xs text-ink-2">{unit}</span></span>
								{:else if send}
									<span class="block digits text-xl leading-tight">{send.grade}</span>
								{/if}
								<span class="mt-0.5 block truncate text-xs font-semibold text-ink-2">
									{session.name} · {scale
										? `${plural(session.climbs.length, 'climb')}, ${session.climbs.filter((c) => c.sent).length} sent`
										: plural(Math.ceil(session.sets.length / sides), 'set')}
								</span>
							</span>
						</svelte:element>
					</li>
				{/each}
			</ul>
		</section>
	{:else}
		<p class="px-1 text-[15px] font-semibold text-ink-2">Nothing logged for this exercise yet.</p>
	{/if}
</div>
