<script lang="ts">
	import FormError from './FormError.svelte';
	import { byDate, fullDate, gridWeeks, monthSpan, weekdayLetters } from './dates';
	import { SessionDrag } from './drag.svelte';
	import type { Cycle, ScheduledDay } from './plan';
	import SessionIcon, { iconState } from './SessionIcon.svelte';
	import { plural } from './text';

	// days holds every row in the cycle's dates, from any cycle. onmove puts a
	// session on another day, and throws when the server refuses.
	let {
		cycle,
		days,
		today,
		cycleNames,
		onmove
	}: {
		cycle: Cycle;
		days: ScheduledDay[];
		today: string;
		cycleNames: Record<string, string>;
		onmove: (d: ScheduledDay, to: string) => Promise<void>;
	} = $props();

	const weeks = $derived(gridWeeks(cycle.starts, cycle.ends));
	const onDate = $derived(byDate(days, (d) => d.local_date));
	const inCycle = (date: string) => date >= cycle.starts && date <= cycle.ends;

	let chosen = $state('');
	const selected = $derived(chosen || (inCycle(today) ? today : cycle.starts));

	const months = $derived(monthSpan(cycle.starts, cycle.ends));

	const look = (d: ScheduledDay) => iconState(d, today);

	// One key entry per session the cycle holds, in the order they first fall.
	const key = $derived([...new Map(days.map((d) => [d.template, d])).values()]);

	const words = { done: 'Done', started: 'Started', paused: 'Paused', missed: 'Missed', planned: '', today: '' };

	let error = $state('');
	const movable = (d: ScheduledDay) => (d.status === 'planned' || d.status === 'paused') && d.local_date >= today;
	const drag = new SessionDrag({
		movable,
		target: (date) => inCycle(date) && date >= today,
		onlift: () => (error = ''),
		ondrop: async (d, to) => {
			chosen = to;
			try {
				await onmove(d, to);
			} catch (e) {
				error = e instanceof Error ? e.message : String(e);
			}
		}
	});
	$effect(() => drag.listen());

	const panel = $derived(drag.over || selected);
</script>

<section class="rounded-3xl bg-surface px-3 pt-4 pb-3 shadow-card" aria-label="Calendar">
	<div class="flex items-baseline justify-between px-1.5">
		<h3 class="text-[15px] font-bold">{months}</h3>
		<span class="text-xs font-semibold text-ink-2">{plural(weeks.length, 'week')}</span>
	</div>
	<ol class="mt-3 grid grid-cols-7 text-center text-xs font-bold text-ink-2" aria-hidden="true">
		{#each weekdayLetters as d, i (i)}
			<li>{d}</li>
		{/each}
	</ol>
	<div class="mt-1 flex flex-col">
		{#each weeks as week (week[0])}
			<div class="grid grid-cols-7">
				{#each week as date (date)}
					{@const list = onDate.get(date) ?? []}
					<button
						type="button"
						data-date={date}
						class="flex min-h-[64px] flex-col items-center gap-1 rounded-2xl pt-1.5 pb-2 transition-colors
							{date === drag.over ? 'bg-tint/40' : date === selected && !drag.lifted ? 'bg-well' : ''}
							{date === today ? 'shadow-[inset_0_0_0_1.5px_var(--ink)]' : ''}
							{drag.lifted && !drag.target(date) && date !== drag.lifted.local_date ? 'opacity-40' : ''}"
						disabled={!inCycle(date)}
						aria-pressed={date === selected}
						aria-label="{fullDate(date)}{list.length
							? `: ${list.map((d) => d.template_name).join(', ')}`
							: ''}"
						onclick={() => {
							if (drag.justDropped) return;
							chosen = date;
							error = '';
						}}
					>
						<span class="text-xs font-bold {inCycle(date) ? (date === today ? 'text-ink' : 'text-ink-2') : 'text-ink-3/50'}">
							{Number(date.slice(8))}
						</span>
						{#each list as d (d.id)}
							<span
								class="rounded-full {movable(d) ? 'select-none [-webkit-touch-callout:none]' : ''} {drag.lifted?.id === d.id ? 'opacity-30' : ''}"
								role="presentation"
								onpointerdown={(e) => drag.grab(e, d)}
								onpointermove={(e) => drag.follow(e)}
								onpointerup={() => drag.drop()}
								onpointercancel={() => drag.cancel()}
								oncontextmenu={(e) => movable(d) && e.preventDefault()}
							>
								<SessionIcon icon={d.template_icon} name={d.template_name} state={look(d)} size={28} />
							</span>
						{/each}
					</button>
				{/each}
			</div>
		{/each}
	</div>

	{#if key.length}
		<ul class="mt-2 flex flex-wrap gap-x-4 gap-y-2 px-1.5 pt-3 shadow-[inset_0_1px_0_var(--line)] text-xs font-semibold text-ink-2">
			{#each key as d (d.template)}
				<li class="flex items-center gap-1.5">
					<SessionIcon icon={d.template_icon} name={d.template_name} size={22} />
					{d.template_name}
				</li>
			{/each}
		</ul>
		<ul class="mt-2 flex flex-wrap gap-x-4 gap-y-2 px-1.5 text-xs font-semibold text-ink-2">
			{#each [['done', 'Done'], ['missed', 'Missed'], ['today', 'Today']] as const as [state, label] (state)}
				<li class="flex items-center gap-1.5">
					<SessionIcon icon={key[0].template_icon} name={key[0].template_name} {state} size={22} />
					{label}
				</li>
			{/each}
		</ul>
	{/if}
</section>

<section
	class="rounded-3xl px-[18px] pt-3.5 pb-2 shadow-card {drag.lifted ? 'bg-tint/25 outline-2 -outline-offset-2 outline-dashed outline-ink-3' : 'bg-surface'}"
	aria-live="polite"
>
	<h3 class="text-[15px] font-bold">{fullDate(panel)}</h3>
	{#if drag.lifted}
		<p class="pt-1 pb-2.5 text-[15px] font-semibold text-ink-2">
			{drag.over ? `Drop to move ${drag.lifted.template_name} here` : 'Drag onto a day from today on'}
		</p>
	{:else if (onDate.get(panel) ?? []).length}
		<ul>
			{#each onDate.get(panel) ?? [] as d (d.id)}
				<li class="flex min-h-[52px] items-center gap-3 py-2 list-row">
					<SessionIcon icon={d.template_icon} name={d.template_name} state={look(d)} size={32} />
					<span class="min-w-0 flex-1">
						<span class="block truncate text-[15px] font-bold">{d.template_name}</span>
						{#if d.cycle !== cycle.id}
							<span class="block truncate text-xs font-semibold text-ink-2">
								{d.cycle ? (cycleNames[d.cycle] ?? 'Another cycle') : d.repeat ? 'Repeats' : 'One-off'}
							</span>
						{/if}
					</span>
					{#if words[look(d)]}
						<span class="shrink-0 text-xs font-bold text-ink-2">{words[look(d)]}</span>
					{/if}
				</li>
			{/each}
		</ul>
	{:else}
		<p class="pt-1 pb-2.5 text-[15px] font-semibold text-ink-2">Rest day</p>
	{/if}
	{#if error && !drag.lifted}
		<div class="pb-2.5"><FormError message={error} /></div>
	{/if}
</section>

{#if drag.lifted}
	<div
		class="pointer-events-none fixed z-50 flex max-w-[60vw] -translate-x-1/2 -translate-y-[calc(100%+12px)] items-center gap-2 rounded-full bg-surface py-1.5 pr-4 pl-1.5 text-[15px] font-bold whitespace-nowrap shadow-card"
		style="left: clamp(30vw, {drag.at.x}px, 70vw); top: {drag.at.y}px"
		aria-hidden="true"
	>
		<SessionIcon icon={drag.lifted.template_icon} name={drag.lifted.template_name} state="today" size={32} />
		<span class="truncate">{drag.lifted.template_name}</span>
	</div>
{/if}
