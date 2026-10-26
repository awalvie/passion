<script lang="ts">
	import { addDays, byDate, formatDate, fullDate, weekdayLetters } from './dates';
	import Icon from './Icon.svelte';
	import type { ScheduledDay } from './plan';

	// Six weeks from the Monday from. Each session is a chip in its own colour
	// from xl, where a day is wide enough for a name, and a dot below that.
	let {
		from,
		today,
		days,
		colors,
		selected,
		onselect
	}: {
		from: string;
		today: string;
		days: ScheduledDay[];
		colors: Map<string, string | null>;
		selected: string;
		onselect: (date: string) => void;
	} = $props();

	const weeks = $derived(Array.from({ length: 6 }, (_, w) => Array.from({ length: 7 }, (_, i) => addDays(from, w * 7 + i))));
	const onDate = $derived(byDate(days, (d) => d.local_date));
	const colorOf = (d: ScheduledDay) => colors.get(d.template) ?? 'var(--ink-3)';

	// A paused session counts in no week.
	function week(dates: string[]) {
		const all = dates.flatMap((date) => onDate.get(date) ?? []);
		const counted = all.filter((d) => d.status !== 'paused');
		return {
			past: dates[0] <= today,
			done: counted.filter((d) => d.status === 'done').length,
			counted: counted.length,
			off: counted.length === 0 && all.length > 0
		};
	}

	const number = (date: string) =>
		date.slice(8) === '01' ? formatDate(date, { day: 'numeric', month: 'short' }) : String(Number(date.slice(8)));

	const look = (d: ScheduledDay) =>
		d.status === 'missed'
			? 'text-ink-2 outline-[1.5px] -outline-offset-[1.5px] outline-dashed outline-ink-3'
			: d.status === 'paused'
				? 'text-ink-3'
				: 'text-ink';
	const chipStyle = (d: ScheduledDay) =>
		d.status === 'missed'
			? ''
			: d.status === 'paused'
				? 'box-shadow: inset 3px 0 0 var(--ink-3)'
				: `background: color-mix(in oklab, ${colorOf(d)} 22%, var(--surface)); box-shadow: inset 3px 0 0 ${colorOf(d)}`;
</script>

<div class="rounded-3xl bg-surface p-1.5 shadow-card xl:grid xl:grid-cols-[1fr_72px] xl:gap-x-2 xl:p-2">
	<div class="grid grid-cols-7">
		{#each weekdayLetters as letter, i (i)}
			<span class="flex h-7 items-center justify-center text-xs font-semibold text-ink-2 xl:h-8 xl:justify-start xl:pl-2.5" aria-hidden="true">
				<span class="xl:hidden">{letter}</span>
				<span class="hidden xl:inline">{formatDate(addDays(from, i), { weekday: 'short' })}</span>
			</span>
		{/each}
		{#each weeks as dates, w (dates[0])}
			{#each dates as date (date)}
				{@const list = onDate.get(date) ?? []}
				<button
					type="button"
					class="flex h-[52px] min-w-0 flex-col items-center justify-center gap-[5px] rounded-xl lg:h-16 xl:h-[92px] xl:items-stretch xl:justify-start xl:gap-1 xl:p-1
						{date === selected ? 'bg-well' : w ? 'xl:rounded-none xl:shadow-[inset_0_1px_0_var(--line)]' : ''}"
					aria-pressed={date === selected}
					aria-label="{fullDate(date)}{list.length ? `: ${list.map((d) => d.template_name).join(', ')}` : ''}"
					onclick={() => onselect(date)}
				>
					<span class="flex h-[26px] shrink-0 items-center gap-1.5 xl:h-7 xl:pl-1">
						<span
							class="digits flex h-[26px] min-w-[26px] items-center justify-center rounded-full text-[15px] whitespace-nowrap {date === today
								? 'bg-tint text-on-tint shadow-[0_0_0_4px_rgba(198,240,91,0.32)] xl:-ml-1'
								: date < today
									? 'text-ink-2'
									: 'text-ink xl:text-ink-2'}"
						>
							{number(date)}
						</span>
					</span>
					<span class="flex h-[7px] gap-[3px] xl:hidden" aria-hidden="true">
						{#each list.slice(0, 4) as d (d.id)}
							<i
								class="size-[7px] rounded-full {d.status === 'paused' ? 'opacity-50' : ''}"
								style={d.status === 'missed'
									? 'box-shadow: inset 0 0 0 1.5px var(--ink-3)'
									: `background: ${d.status === 'paused' ? 'var(--ink-3)' : colorOf(d)}`}
							></i>
						{/each}
					</span>
					{#each list.length > 2 ? list.slice(0, 1) : list as d (d.id)}
						<span class="hidden h-6 min-w-0 items-center gap-1 rounded-[7px] pr-1.5 pl-2 text-xs font-bold xl:flex {look(d)}" style={chipStyle(d)}>
							<span class="min-w-0 flex-1 truncate text-left {d.status === 'paused' ? 'line-through' : ''}">{d.template_name}</span>
							{#if d.repeat}<span class="text-ink-2"><Icon name="repeat" size="12px" stroke={2.6} /></span>{/if}
							{#if d.status === 'done'}<span class="text-ink-2"><Icon name="check" size="12px" stroke={3} /></span>{/if}
						</span>
					{/each}
					{#if list.length > 2}
						<span class="hidden h-6 items-center pl-2 text-xs font-bold text-ink-2 xl:flex">+{list.length - 1} more</span>
					{/if}
				</button>
			{/each}
		{/each}
	</div>

	<div class="hidden grid-rows-[2rem_repeat(6,92px)] rounded-2xl bg-well xl:grid" aria-label="Weeks">
		<span class="flex items-center justify-center text-xs font-semibold text-ink-2">Week</span>
		{#each weeks as dates, w (dates[0])}
			{@const s = week(dates)}
			<span class="flex flex-col items-center justify-center gap-1.5 {w ? 'shadow-[inset_0_1px_0_var(--line)]' : ''}">
				{#if s.off}
					<span class="text-xs font-bold text-ink-2">Off</span>
				{:else if s.counted && s.past}
					<b class="digits text-xl leading-none" aria-label="{s.done} of {s.counted} done">{s.done}<small class="text-[15px] text-ink-2">/{s.counted}</small></b>
					<span class="h-[5px] w-11 overflow-hidden rounded-full bg-ink/12" aria-hidden="true">
						<i class="block h-full bg-ink" style:width="{(s.done / s.counted) * 100}%"></i>
					</span>
				{:else if s.counted}
					<b class="digits text-xl leading-none text-ink-2" aria-label="{s.counted} planned">{s.counted}</b>
				{/if}
			</span>
		{/each}
	</div>
</div>

<ul class="hidden flex-wrap gap-x-[18px] gap-y-2 px-1.5 pt-3 text-xs font-semibold text-ink-2 xl:flex">
	<li class="flex items-center gap-1.5">
		<span class="flex h-6 w-7 items-center justify-center rounded-[7px] text-ink-2" style={chipStyle({ status: 'done', template: '' } as ScheduledDay)}>
			<Icon name="check" size="12px" stroke={3} />
		</span>Done
	</li>
	<li class="flex items-center gap-1.5">
		<span class="h-6 w-7 rounded-[7px] {look({ status: 'missed' } as ScheduledDay)}"></span>Missed
	</li>
	<li class="flex items-center gap-1.5">
		<span class="h-6 w-7 rounded-[7px]" style={chipStyle({ status: 'paused' } as ScheduledDay)}></span>Paused
	</li>
	<li class="flex items-center gap-1.5">
		<span class="flex h-6 w-7 items-center justify-center rounded-[7px] text-ink-2" style={chipStyle({ status: 'planned', template: '' } as ScheduledDay)}>
			<Icon name="repeat" size="12px" stroke={2.6} />
		</span>Repeats
	</li>
</ul>
