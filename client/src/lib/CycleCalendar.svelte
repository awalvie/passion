<script lang="ts">
	import { formatDate, gridWeeks } from './dates';
	import type { Cycle, ScheduledDay } from './plan';
	import SessionIcon, { type IconState } from './SessionIcon.svelte';

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
	const byDate = $derived.by(() => {
		const m = new Map<string, ScheduledDay[]>();
		for (const d of days) m.set(d.local_date, [...(m.get(d.local_date) ?? []), d]);
		return m;
	});
	const inCycle = (date: string) => date >= cycle.starts && date <= cycle.ends;

	let chosen = $state('');
	const selected = $derived(chosen || (inCycle(today) ? today : cycle.starts));

	const months = $derived.by(() => {
		const a = formatDate(cycle.starts, { month: 'long' });
		const b = formatDate(cycle.ends, { month: 'long' });
		return a === b ? a : `${a} – ${b}`;
	});

	function look(d: ScheduledDay): IconState {
		if (d.status !== 'planned') return d.status;
		return d.local_date === today ? 'today' : 'planned';
	}

	// One key entry per session the cycle holds, in the order they first fall.
	const key = $derived([...new Map(days.map((d) => [d.template, d])).values()]);

	const words = { done: 'Done', started: 'Started', missed: 'Missed', planned: '', today: '' };

	// A hold on a planned session lifts it; it then follows the finger, and a
	// drop on a day from today on moves it there. A move before the hold ends
	// is a scroll, so the hold gives up.
	let lifted = $state<ScheduledDay | null>(null);
	let at = $state({ x: 0, y: 0 });
	let over = $state('');
	let error = $state('');
	let hold: ReturnType<typeof setTimeout> | undefined;
	let start = { x: 0, y: 0 };
	// The click that ends a drag lands on the day it started from, sometimes
	// well after the drop; it must not choose that day.
	let droppedAt = 0;

	const movable = (d: ScheduledDay) => d.status === 'planned' && d.local_date >= today;
	const target = (date: string) => inCycle(date) && date >= today && date !== lifted?.local_date;

	// Icons keep touch scrolling, so a swipe that starts on one still scrolls.
	// Once a session is lifted, the page must not scroll under the finger.
	$effect(() => {
		const stop = (e: TouchEvent) => lifted && e.preventDefault();
		window.addEventListener('touchmove', stop, { passive: false });
		return () => window.removeEventListener('touchmove', stop);
	});

	function grab(e: PointerEvent, d: ScheduledDay) {
		if (e.button > 0 || !movable(d)) return;
		const el = e.currentTarget as Element;
		const id = e.pointerId;
		start = { x: e.clientX, y: e.clientY };
		at = start;
		clearTimeout(hold);
		hold = setTimeout(() => {
			lifted = d;
			over = '';
			error = '';
			el.setPointerCapture(id);
			navigator.vibrate?.(10);
		}, 300);
	}

	function follow(e: PointerEvent) {
		if (!lifted) {
			if (Math.hypot(e.clientX - start.x, e.clientY - start.y) > 8) clearTimeout(hold);
			return;
		}
		at = { x: e.clientX, y: e.clientY };
		const cell = document.elementFromPoint(e.clientX, e.clientY)?.closest<HTMLElement>('[data-date]');
		const date = cell?.dataset.date ?? '';
		over = target(date) ? date : '';
	}

	async function drop() {
		clearTimeout(hold);
		const d = lifted;
		const to = over;
		lifted = null;
		over = '';
		if (d) droppedAt = performance.now();
		if (!d || !to) return;
		chosen = to;
		try {
			await onmove(d, to);
		} catch (e) {
			error = e instanceof Error ? e.message : String(e);
		}
	}

	function cancel() {
		clearTimeout(hold);
		lifted = null;
		over = '';
	}

	const panel = $derived(over || selected);
</script>

<section class="rounded-3xl bg-surface px-3 pt-4 pb-3 shadow-card" aria-label="Calendar">
	<div class="flex items-baseline justify-between px-1.5">
		<h3 class="text-[15px] font-bold">{months}</h3>
		<span class="text-xs font-semibold text-ink-2">{weeks.length === 1 ? '1 week' : `${weeks.length} weeks`}</span>
	</div>
	<ol class="mt-3 grid grid-cols-7 text-center text-xs font-bold text-ink-2" aria-hidden="true">
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
						data-date={date}
						class="flex min-h-[64px] flex-col items-center gap-1 rounded-2xl pt-1.5 pb-2 transition-colors
							{date === over ? 'bg-tint/40' : date === selected && !lifted ? 'bg-well' : ''}
							{date === today ? 'shadow-[inset_0_0_0_1.5px_var(--ink)]' : ''}
							{lifted && !target(date) && date !== lifted.local_date ? 'opacity-40' : ''}"
						disabled={!inCycle(date)}
						aria-pressed={date === selected}
						aria-label="{formatDate(date, { weekday: 'long', day: 'numeric', month: 'long' })}{list.length
							? `: ${list.map((d) => d.template_name).join(', ')}`
							: ''}"
						onclick={() => {
							if (performance.now() - droppedAt < 500) return;
							chosen = date;
							error = '';
						}}
					>
						<span class="text-xs font-bold {inCycle(date) ? (date === today ? 'text-ink' : 'text-ink-2') : 'text-ink-3/50'}">
							{Number(date.slice(8))}
						</span>
						{#each list as d (d.id)}
							<span
								class="rounded-full {movable(d) ? 'select-none [-webkit-touch-callout:none]' : ''} {lifted?.id === d.id ? 'opacity-30' : ''}"
								role="presentation"
								onpointerdown={(e) => grab(e, d)}
								onpointermove={follow}
								onpointerup={drop}
								onpointercancel={cancel}
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
		<ul class="mt-2 flex flex-wrap gap-x-4 gap-y-2 border-t border-line px-1.5 pt-3 text-xs font-semibold text-ink-2">
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
	class="rounded-3xl px-[18px] pt-3.5 pb-2 shadow-card {lifted ? 'bg-tint/25 outline-2 -outline-offset-2 outline-dashed outline-ink-3' : 'bg-surface'}"
	aria-live="polite"
>
	<h3 class="text-[15px] font-bold">{formatDate(panel, { weekday: 'long', day: 'numeric', month: 'long' })}</h3>
	{#if lifted}
		<p class="pt-1 pb-2.5 text-[15px] font-semibold text-ink-2">
			{over ? `Drop to move ${lifted.template_name} here` : 'Drag onto a day from today on'}
		</p>
	{:else if (byDate.get(panel) ?? []).length}
		<ul>
			{#each byDate.get(panel) ?? [] as d (d.id)}
				<li class="flex min-h-[52px] items-center gap-3 py-2 [&:not(:first-child)]:shadow-[inset_0_1px_0_var(--line)]">
					<SessionIcon icon={d.template_icon} name={d.template_name} state={look(d)} size={32} />
					<span class="min-w-0 flex-1">
						<span class="block truncate text-[15px] font-bold">{d.template_name}</span>
						{#if d.cycle !== cycle.id}
							<span class="block truncate text-xs font-semibold text-ink-2">
								{d.cycle ? (cycleNames[d.cycle] ?? 'Another cycle') : 'One-off'}
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
	{#if error && !lifted}
		<p class="pb-2.5 text-[15px] font-semibold text-bad">{error}</p>
	{/if}
</section>

{#if lifted}
	<div
		class="pointer-events-none fixed z-50 flex max-w-[60vw] -translate-x-1/2 -translate-y-[calc(100%+12px)] items-center gap-2 rounded-full bg-surface py-1.5 pr-4 pl-1.5 text-[15px] font-bold whitespace-nowrap shadow-card"
		style="left: clamp(30vw, {at.x}px, 70vw); top: {at.y}px"
		aria-hidden="true"
	>
		<SessionIcon icon={lifted.template_icon} name={lifted.template_name} state="today" size={32} />
		<span class="truncate">{lifted.template_name}</span>
	</div>
{/if}
