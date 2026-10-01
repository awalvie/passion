<script lang="ts">
	import { gridWeeks } from './dates';
	import type { Cycle, ScheduledDay } from './plan';
	import SessionIcon, { type IconState } from './SessionIcon.svelte';

	let { cycle, days, today }: { cycle: Cycle; days: ScheduledDay[]; today: string } = $props();

	const utc = (date: string, o: Intl.DateTimeFormatOptions) =>
		new Date(`${date}T00:00:00Z`).toLocaleDateString(undefined, { ...o, timeZone: 'UTC' });

	const weeks = $derived(gridWeeks(cycle.starts, cycle.ends));
	const byDate = $derived.by(() => {
		const m = new Map<string, ScheduledDay[]>();
		for (const d of days) m.set(d.local_date, [...(m.get(d.local_date) ?? []), d]);
		return m;
	});
	const inCycle = (date: string) => date >= cycle.starts && date <= cycle.ends;

	let chosen = $state('');
	const selected = $derived(chosen || (inCycle(today) ? today : cycle.starts));

	const months = $derived.by(() => {
		const a = utc(cycle.starts, { month: 'long' });
		const b = utc(cycle.ends, { month: 'long' });
		return a === b ? a : `${a} – ${b}`;
	});

	function look(d: ScheduledDay): IconState {
		if (d.status !== 'planned') return d.status;
		return d.local_date === today ? 'today' : 'planned';
	}

	// One key entry per session the cycle holds, in the order they first fall.
	const key = $derived([...new Map(days.map((d) => [d.template, d])).values()]);

	const words = { done: 'Done', started: 'Started', missed: 'Missed', planned: '', today: '' };
</script>

<section class="rounded-3xl bg-surface px-3 pt-4 pb-3 shadow-card" aria-label="Calendar">
	<div class="flex items-baseline justify-between px-1.5">
		<h3 class="text-[15px] font-bold">{months}</h3>
		<span class="text-xs font-semibold text-ink-2">{weeks.length === 1 ? '1 week' : `${weeks.length} weeks`}</span>
	</div>
	<ol class="mt-3 grid grid-cols-7 text-center text-xs font-bold text-ink-3" aria-hidden="true">
		{#each ['M', 'T', 'W', 'T', 'F', 'S', 'S'] as d, i (i)}
			<li>{d}</li>
		{/each}
	</ol>
	<div class="mt-1 flex flex-col">
		{#each weeks as week (week[0])}
			<div class="grid grid-cols-7">
				{#each week as date (date)}
					{@const list = byDate.get(date) ?? []}
					<button
						type="button"
						class="flex min-h-[64px] flex-col items-center gap-1 rounded-2xl pt-1.5 pb-2
							{date === selected ? 'bg-well' : ''} {date === today ? 'shadow-[inset_0_0_0_1.5px_var(--tint)]' : ''}"
						disabled={!inCycle(date)}
						aria-pressed={date === selected}
						aria-label="{utc(date, { weekday: 'long', day: 'numeric', month: 'long' })}{list.length
							? `: ${list.map((d) => d.template_name).join(', ')}`
							: ''}"
						onclick={() => (chosen = date)}
					>
						<span class="text-xs font-bold {inCycle(date) ? (date === today ? 'text-ink' : 'text-ink-2') : 'text-ink-3/50'}">
							{Number(date.slice(8))}
						</span>
						{#each list as d (d.id)}
							<SessionIcon icon={d.template_icon} name={d.template_name} state={look(d)} size={28} />
						{/each}
					</button>
				{/each}
			</div>
		{/each}
	</div>

	{#if key.length}
		<ul class="mt-2 flex flex-wrap gap-x-4 gap-y-2 border-t border-line px-1.5 pt-3 text-xs font-semibold text-ink-2">
			{#each key as d (d.template)}
				<li class="flex items-center gap-1.5">
					<SessionIcon icon={d.template_icon} name={d.template_name} size={22} />
					{d.template_name}
				</li>
			{/each}
		</ul>
	{/if}
</section>

<section class="rounded-3xl bg-surface px-[18px] pt-3.5 pb-2 shadow-card" aria-live="polite">
	<h3 class="text-[15px] font-bold">{utc(selected, { weekday: 'long', day: 'numeric', month: 'long' })}</h3>
	{#if (byDate.get(selected) ?? []).length}
		<ul>
			{#each byDate.get(selected) ?? [] as d (d.id)}
				<li class="flex min-h-[52px] items-center gap-3 py-2 [&:not(:first-child)]:shadow-[inset_0_1px_0_var(--line)]">
					<SessionIcon icon={d.template_icon} name={d.template_name} state={look(d)} size={32} />
					<span class="min-w-0 flex-1 truncate text-[15px] font-bold">{d.template_name}</span>
					{#if words[look(d)]}
						<span class="shrink-0 text-xs font-bold text-ink-2">{words[look(d)]}</span>
					{/if}
				</li>
			{/each}
		</ul>
	{:else}
		<p class="pt-1 pb-2.5 text-[15px] font-semibold text-ink-2">Rest day</p>
	{/if}
</section>
