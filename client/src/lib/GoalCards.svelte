<script lang="ts">
	import { invalidateAll } from '$app/navigation';
	import { describe, request } from './api';
	import Button from './Button.svelte';
	import Checkbox from './Checkbox.svelte';
	import { cycleWeek, shortDate } from './dates';
	import Empty from './Empty.svelte';
	import FormError from './FormError.svelte';
	import { current, goalPeriod, periodOf, periodTitle, spanTitle, through, ticked, type GoalSpan, type PeriodGoal } from './goals';
	import Icon from './Icon.svelte';
	import PeriodGoalSheet from './PeriodGoalSheet.svelte';
	import { saveCycle, type Cycle } from './plan';

	// Today's goals, one card a period: this week, each running cycle and span
	// by its first day, this month, this year.
	let { goals, cycles, today }: { goals: PeriodGoal[]; cycles: Cycle[]; today: string } = $props();

	type Row = { text: string; done: boolean; tick: (done: boolean) => Promise<unknown>; edit?: () => void; href?: string };
	type Card = { key: string; title: string; note: string; through: number; rows: Row[]; starts?: string; span?: GoalSpan; waits?: boolean };

	const rowOf = (g: PeriodGoal): Row => ({
		text: g.text,
		done: ticked(g, today),
		tick: (done) => request(done ? 'PUT' : 'DELETE', `/api/v1/goals/${g.id}/ticks/${goalPeriod(g, today).from}`),
		edit: () => edit(g)
	});

	const cards = $derived.by(() => {
		const live = current(goals, today);
		const repeating = (['week', 'month', 'year'] as const).map((period): Card => {
			const { from, to } = periodOf(period, today);
			return {
				key: period,
				...periodTitle(period, today),
				through: through(from, to, today),
				rows: live.filter((g) => g.period === period).map(rowOf)
			};
		});
		const running = cycles
			.filter((c) => c.starts <= today && today <= c.ends && c.goals.length)
			.map((c): Card => {
				const { week, of } = cycleWeek(c.starts, c.ends, today);
				return {
					key: c.id,
					starts: c.starts,
					title: c.name,
					note: `Cycle · week ${week} of ${of} · ends ${shortDate(c.ends)}`,
					through: through(c.starts, c.ends, today),
					rows: c.goals.map((x, i) => ({
						text: x.text,
						done: x.done,
						href: `/plan/cycles/${c.id}`,
						tick: (done) => saveCycle({ ...c, goals: c.goals.map((y, j) => (j === i ? { ...y, done } : y)) })
					}))
				};
			});
		const spans = new Map<string, Card>();
		for (const g of live) {
			if (!g.span) continue;
			const s = g.span;
			if (!spans.has(s.id)) {
				spans.set(s.id, { key: s.id, starts: s.starts, span: s, waits: s.starts > today, ...spanTitle(s, today), through: through(s.starts, s.ends, today), rows: [] });
			}
			spans.get(s.id)!.rows.push(rowOf(g));
		}
		const between = [...running, ...spans.values()].sort((a, b) => a.starts!.localeCompare(b.starts!));
		return [repeating[0], ...between, repeating[1], repeating[2]].filter((c) => c.rows.length);
	});

	let open = $state(false);
	let editing = $state<PeriodGoal | null>(null);
	let span = $state<GoalSpan | null>(null);
	function edit(g: PeriodGoal) {
		editing = g;
		span = null;
		open = true;
	}
	function add(s: GoalSpan | null) {
		editing = null;
		span = s;
		open = true;
	}

	// One tick at a time, so a second tap cannot send a cycle's goals as they
	// were before the first. A tap meanwhile puts its box back.
	let busy = false;
	let error = $state('');
	async function tick(r: Row, box: HTMLInputElement) {
		if (busy) {
			box.checked = !box.checked;
			return;
		}
		const done = box.checked;
		busy = true;
		error = '';
		try {
			await r.tick(done);
		} catch (e) {
			error = describe(e, (f) => f);
		}
		await invalidateAll();
		busy = false;
	}
</script>

<div class="mx-4 mt-6 flex items-center justify-between px-1">
	<a href="/plan?view=goals" class="-ml-1 flex h-11 items-center gap-1 rounded-full px-1 text-xl font-bold" aria-label="All goals">
		Goals<span class="text-ink-3"><Icon name="chevron-right" size="1rem" /></span>
	</a>
	{#if cards.length}
		<button type="button" class="-mr-3 flex h-11 items-center gap-1 rounded-full px-3 text-[15px] font-bold" onclick={() => add(null)}>
			<Icon name="plus" size="1rem" />Add
		</button>
	{/if}
</div>
<div class="px-4"><FormError message={error} /></div>

{#if !cards.length}
	<div class="mx-4 mt-2">
		<Empty title="No goals yet" text="Aim for a week, a month, a year or days you pick.">
			<Button onclick={() => add(null)}>Add goal</Button>
		</Empty>
	</div>
{/if}

{#each cards as c, i (c.key)}
	<section class="mx-4 {i ? 'mt-3.5' : 'mt-2'} rounded-3xl bg-surface px-[18px] pt-4 pb-1 shadow-card">
		<div class="flex items-baseline justify-between gap-3">
			<h3 class="min-w-0 truncate text-[15px] font-extrabold">{c.title}</h3>
			<p class="shrink-0 text-xs font-semibold text-ink-2">
				<b class="digits text-[15px] text-ink">{c.rows.filter((r) => r.done).length} of {c.rows.length}</b> done
			</p>
		</div>
		<p class="mt-0.5 text-xs font-semibold text-ink-2">{c.note}</p>
		<div class="mt-2 mb-1 h-1 overflow-hidden rounded-full bg-well">
			<div class="h-full rounded-full bg-ink-2" style:width="{c.through * 100}%"></div>
		</div>
		<ul>
			{#each c.rows as r, j (j)}
				<li class="flex min-h-[52px] items-center gap-1 list-row">
					<label class="-ml-2.5 flex size-11 shrink-0 cursor-pointer items-center justify-center has-[:disabled]:cursor-default">
						<Checkbox checked={r.done} label={r.text} disabled={c.waits} onchange={(e) => tick(r, e.currentTarget)} />
					</label>
					{#if r.href}
						<a href={r.href} class="flex min-h-11 min-w-0 flex-1 items-center gap-2 py-1.5">
							<span class="min-w-0 flex-1 text-[15px] font-bold break-words {r.done ? 'text-ink-2 line-through' : ''}">{r.text}</span>
							<span class="shrink-0 text-ink-3"><Icon name="chevron-right" size="1rem" /></span>
						</a>
					{:else}
						<button type="button" class="flex min-h-11 min-w-0 flex-1 items-center py-1.5 text-left" onclick={r.edit}>
							<span class="text-[15px] font-bold break-words {r.done ? 'text-ink-2 line-through' : ''}">{r.text}</span>
						</button>
					{/if}
				</li>
			{/each}
			{#if c.span}
				<li class="list-row">
					<button type="button" class="flex min-h-[52px] w-full items-center gap-3 text-[15px] font-bold text-ink-2" onclick={() => add(c.span!)}>
						<span class="flex size-6 shrink-0 items-center justify-center text-ink-3"><Icon name="plus" size="1.125rem" /></span>
						Add goal
					</button>
				</li>
			{/if}
		</ul>
	</section>
{/each}

<PeriodGoalSheet bind:open {editing} {span} {goals} {today} />
