<script lang="ts">
	import Button from './Button.svelte';
	import { shortDate } from './dates';
	import Empty from './Empty.svelte';
	import { current, pastPeriods, type PeriodGoal } from './goals';
	import Icon from './Icon.svelte';
	import PastGoals from './PastGoals.svelte';
	import type { Cycle } from './plan';
	import { plural } from './text';

	// Plan's Goals: the goals running now, to change, then each period that has
	// ended. From lg a past period opens in the pane; below it, on its own page.
	let {
		goals,
		cycles,
		today,
		wide,
		onedit,
		onadd
	}: { goals: PeriodGoal[]; cycles: Cycle[]; today: string; wide: boolean; onedit: (g: PeriodGoal) => void; onadd: () => void } = $props();

	type Row = { key: string; text: string; note: string; starts: string; rank: number; goal?: PeriodGoal; href?: string; done?: boolean };

	// In the order Today shows them: the week, cycles and days by their first
	// day, the month, the year.
	const now = $derived.by(() => {
		const rank = { week: 0, custom: 1, month: 2, year: 3 };
		const rows: Row[] = current(goals, today).map((g) => {
			const s = g.span;
			const days = s ? `${shortDate(s.starts)} – ${shortDate(s.ends)}` : '';
			return { key: g.id, text: g.text, note: s ? (s.name ? `${s.name} · ${days}` : days) : `Every ${g.period}`, starts: s?.starts ?? '', rank: rank[g.period], goal: g };
		});
		for (const c of cycles.filter((c) => c.starts <= today && today <= c.ends)) {
			c.goals.forEach((x, i) =>
				rows.push({ key: `${c.id}-${i}`, text: x.text, note: `${c.name} · ${shortDate(c.starts)} – ${shortDate(c.ends)}`, starts: c.starts, rank: 1, href: `/plan/cycles/${c.id}`, done: x.done })
			);
		}
		return rows.sort((a, b) => a.rank - b.rank || a.starts.localeCompare(b.starts));
	});
	const past = $derived(pastPeriods(goals, today));

	let pickedKey = $state('');
	const picked = $derived(past.find((p) => p.key === pickedKey) ?? past[0]);
</script>

{#snippet pastRow(p: (typeof past)[number])}
	<span class="min-w-0 flex-1">
		<span class="block truncate text-[15px] font-bold">{p.title}</span>
		<span class="mt-0.5 block truncate text-xs font-semibold text-ink-2">{p.note}</span>
	</span>
	<span class="shrink-0 text-xs font-semibold text-ink-2">
		<b class="digits text-[15px] text-ink">{p.goals.filter((g) => g.done).length} of {p.goals.length}</b> done
	</span>
	{#if !wide}<span class="shrink-0 text-ink-3"><Icon name="chevron-right" size="1rem" /></span>{/if}
{/snippet}

<div class="flex flex-col gap-3.5 lg:mt-2 lg:grid lg:grid-cols-[minmax(0,1fr)_330px] lg:items-start lg:gap-[22px]">
	<div class="flex flex-col gap-3.5">
		{#if now.length}
			<div class="flex items-baseline justify-between gap-3 px-1 pt-1.5 lg:h-11 lg:items-center lg:pt-0">
				<h2 class="text-xl font-bold">Now</h2>
				<span class="text-xs font-semibold text-ink-2">{plural(now.length, 'goal')}</span>
			</div>
			<ul class="rounded-3xl bg-surface px-[18px] py-1 shadow-card">
				{#each now as r (r.key)}
					<li class="list-row">
						{#snippet body()}
							<span class="min-w-0 flex-1">
								<span class="block text-[15px] font-bold break-words">{r.text}</span>
								<span class="mt-0.5 block truncate text-xs font-semibold text-ink-2">{r.note}</span>
							</span>
						{/snippet}
						{#if r.href}
							<a href={r.href} class="flex min-h-[58px] items-center gap-3 py-2">
								{@render body()}
								{#if r.done}<span class="shrink-0 rounded-xl bg-well px-2.5 py-[5px] text-xs font-bold text-ink-2">Done</span>{/if}
								<span class="shrink-0 text-ink-3"><Icon name="chevron-right" size="1rem" /></span>
							</a>
						{:else}
							<button type="button" class="flex min-h-[58px] w-full items-center gap-3 py-2 text-left" onclick={() => onedit(r.goal!)}>
								{@render body()}
							</button>
						{/if}
					</li>
				{/each}
			</ul>
		{:else}
			<Empty title={past.length ? 'Nothing running' : 'No goals yet'} text="Aim for a week, a month, a year or days you pick.">
				<!-- From lg the header's New goal is the one button. -->
				<div class="lg:hidden"><Button onclick={onadd}>New goal</Button></div>
			</Empty>
		{/if}

		{#if past.length}
			<h2 class="px-1 pt-1.5 text-xl font-bold">Past</h2>
			<ul class="rounded-3xl bg-surface px-[18px] py-1 shadow-card">
				{#each past as p (p.key)}
					<li class="list-row">
						{#if wide}
							<button
								type="button"
								class="-mx-2.5 flex min-h-[58px] w-[calc(100%+1.25rem)] items-center gap-3 rounded-2xl px-2.5 py-2 text-left {p.key === picked?.key ? 'bg-well' : ''}"
								aria-pressed={p.key === picked?.key}
								onclick={() => (pickedKey = p.key)}
							>
								{@render pastRow(p)}
							</button>
						{:else}
							<a href="/plan/goals/{p.key}" class="flex min-h-[58px] items-center gap-3 py-2">{@render pastRow(p)}</a>
						{/if}
					</li>
				{/each}
			</ul>
		{/if}
	</div>
	{#if wide && picked}
		<section class="sticky top-6 flex flex-col gap-2" aria-live="polite">
			<div class="flex h-11 items-center justify-between gap-3 px-1">
				<h2 class="truncate text-[15px] font-bold">{picked.title}</h2>
				<span class="shrink-0 text-xs font-semibold text-ink-2">
					{picked.note} · {picked.goals.filter((g) => g.done).length} of {picked.goals.length} done
				</span>
			</div>
			<PastGoals period={picked} />
		</section>
	{/if}
</div>
