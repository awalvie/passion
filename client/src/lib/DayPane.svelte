<script lang="ts">
	import type { DayActions } from './dayActions.svelte';
	import { dayDate, formatDate, shortDate } from './dates';
	import FormError from './FormError.svelte';
	import Icon from './Icon.svelte';
	import Menu from './Menu.svelte';
	import { statusWords, type ScheduledDay, type TimeOff } from './plan';
	import SessionIcon, { iconState } from './SessionIcon.svelte';

	// One day's sessions, each with Start or its menu, then the time off in
	// view.
	let {
		date,
		today,
		days,
		live,
		day,
		cycleNames,
		timeOff,
		onoff
	}: {
		date: string;
		today: string;
		days: ScheduledDay[];
		live: boolean;
		day: DayActions;
		cycleNames: Map<string, string>;
		timeOff: TimeOff[];
		onoff: (o: TimeOff) => void;
	} = $props();

	const offIcons = { trip: 'plane', injury: 'bandage', illness: 'heart-pulse', other: 'calendar' } as const;
	const offKinds = { trip: 'Trip', injury: 'Injury', illness: 'Illness', other: 'Other' };
	const source = (d: ScheduledDay) =>
		[statusWords[d.status], d.cycle ? (cycleNames.get(d.cycle) ?? 'Cycle') : d.repeat ? 'Repeats' : 'One-off'].filter(Boolean).join(' · ');
</script>

<section class="flex flex-col gap-2" aria-live="polite">
	<div class="flex h-11 items-center justify-between px-1">
		<h2 class="text-[15px] font-bold">{date === today ? 'Today' : formatDate(date, { weekday: 'long' })}</h2>
		<span class="text-xs font-semibold text-ink-2">{dayDate(date)}</span>
	</div>
	<div class="rounded-3xl bg-surface px-[18px] py-1 shadow-card">
		{#if days.length}
			<ul>
				{#each days as d (d.id)}
					<li class="list-row flex min-h-[58px] items-center gap-3 py-2">
						<SessionIcon icon={d.template_icon} name={d.template_name} state={iconState(d, today)} size={36} />
						<span class="min-w-0 flex-1">
							<span class="line-clamp-2 text-[15px] font-bold {d.status === 'missed' || d.status === 'paused' ? 'text-ink-2' : ''}">{d.template_name}</span>
							<span class="mt-0.5 block truncate text-xs font-semibold text-ink-2">{source(d)}</span>
						</span>
						{#if d.run}
							<a href={d.status === 'done' ? `/history/${d.run}` : `/run/${d.run}`} class="flex size-11 shrink-0 items-center justify-center text-ink-3" aria-label="Open {d.template_name}">
								<Icon name="chevron-right" size="1rem" />
							</a>
						{:else}
							{#if (d.status === 'planned' || d.status === 'paused') && d.local_date === today && !live}
								<button
									type="button"
									class="flex size-11 shrink-0 items-center justify-center rounded-full bg-tint text-on-tint shadow-tint disabled:opacity-50"
									aria-label="Start {d.template_name}"
									disabled={day.starting}
									onclick={() => day.start(d)}
								>
									<Icon name="play" size="1.125rem" />
								</button>
							{/if}
							<Menu
								label="More for {d.template_name}"
								look="text-ink-2"
								items={[
									{ label: 'Move to another day', onclick: () => day.openMove(d) },
									{ label: 'Remove', danger: true, onclick: () => day.remove(d) }
								]}
							/>
						{/if}
					</li>
				{/each}
			</ul>
		{:else}
			<p class="py-4 text-[15px] font-semibold text-ink-2">Nothing planned</p>
		{/if}
	</div>
	<FormError message={days.some((d) => d.id === day.error.id) && !day.moveOpen && !day.endOpen ? day.error.message : ''} />
	{#if timeOff.length}
		<h2 class="flex h-11 items-center px-1 pt-2 text-[15px] font-bold">Time off</h2>
		<ul class="rounded-3xl bg-surface px-[18px] py-1 shadow-card">
			{#each timeOff as o (o.id)}
				<li class="list-row">
					<button type="button" class="flex min-h-[58px] w-full items-center gap-3 py-2 text-left" onclick={() => onoff(o)}>
						<span class="flex size-9 shrink-0 items-center justify-center rounded-full bg-well text-ink-2"><Icon name={offIcons[o.kind]} size="1.125rem" /></span>
						<span class="min-w-0 flex-1">
							<span class="block truncate text-[15px] font-bold">{o.name}</span>
							<span class="mt-0.5 block truncate text-xs font-semibold text-ink-2">
								{offKinds[o.kind]} · {o.ends ? `${shortDate(o.starts)} – ${shortDate(o.ends)}` : `from ${shortDate(o.starts)}`}
							</span>
						</span>
						<span class="shrink-0 text-ink-3"><Icon name="chevron-right" size="1rem" /></span>
					</button>
				</li>
			{/each}
		</ul>
	{/if}
</section>
