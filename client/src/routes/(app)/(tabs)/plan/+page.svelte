<script lang="ts">
	import AddSession from '$lib/AddSession.svelte';
	import Segmented from '$lib/Segmented.svelte';
	import { page } from '$app/state';
	import { MediaQuery } from 'svelte/reactivity';
	import CycleYear from '$lib/CycleYear.svelte';
	import { addDays, cycleWeek, daysBetween, formatDate, fullDate, mondayOf, monthSpan, shortDate, weekday } from '$lib/dates';
	import { DayActions } from '$lib/dayActions.svelte';
	import DayPane from '$lib/DayPane.svelte';
	import DaySheets from '$lib/DaySheets.svelte';
	import DateField from '$lib/DateField.svelte';
	import FormError from '$lib/FormError.svelte';
	import Icon from '$lib/Icon.svelte';
	import Menu from '$lib/Menu.svelte';
	import PlanCalendar from '$lib/PlanCalendar.svelte';
	import SessionIcon, { iconState } from '$lib/SessionIcon.svelte';
	import { statusWords, type ScheduledDay, type TimeOff } from '$lib/plan';
	import TimeOffSheet from '$lib/TimeOffSheet.svelte';
	import WeekStrip from '$lib/WeekStrip.svelte';

	let { data } = $props();

	const view = $derived(page.url.searchParams.get('view'));
	const cyclesView = $derived(view === 'cycles');
	const calendarView = $derived(view === 'calendar');
	// From lg a Schedule row picks its day for the pane beside the list.
	const wide = new MediaQuery('min-width: 64rem');

	let adding = $state(false);
	const openAdd = () => (adding = true);
	let offOpen = $state(false);
	let editingOff = $state<TimeOff | null>(null);
	function openOff(o: TimeOff | null) {
		editingOff = o;
		offOpen = true;
	}
	// The time off from the first week shown on.
	const shownOff = $derived(data.timeOff.filter((o) => !o.ends || o.ends >= data.from));
	const day = new DayActions();

	const cycleNames = $derived(new Map(data.cycles.map((c) => [c.id, c.name])));
	const templates = $derived(new Map(data.templates.map((t) => [t.id, t])));
	const colors = $derived(new Map(data.templates.map((t) => [t.id, t.color])));

	// The calendar's day: the one picked, else today when it shows, else its
	// first day.
	let chosen = $state('');
	const last = $derived(addDays(data.from, 41));
	const inView = (date: string) => date >= data.from && date <= last;
	const selected = $derived(inView(chosen) ? chosen : inView(data.today) ? data.today : data.from);
	const picked = $derived(data.days.filter((d) => d.local_date === selected));
	// A refused move leaves the session where it was, so its day shows why.
	async function dragged(d: ScheduledDay, to: string) {
		await day.move(d, to);
		if (day.error.id === d.id) chosen = d.local_date;
	}
	const months = $derived(monthSpan(data.from, last));
	const calendarAt = (from: string) => `/plan?view=calendar&from=${from}`;

	const weeks = $derived.by(() => {
		const out: { monday: string; dates: { date: string; days: ScheduledDay[] }[] }[] = [];
		for (const d of data.days) {
			const monday = mondayOf(d.local_date);
			if (out.at(-1)?.monday !== monday) out.push({ monday, dates: [] });
			const dates = out.at(-1)!.dates;
			if (dates.at(-1)?.date !== d.local_date) dates.push({ date: d.local_date, days: [] });
			dates.at(-1)!.days.push(d);
		}
		return out;
	});

	function weekTitle(monday: string) {
		const now = mondayOf(data.today);
		if (monday === now) return 'This week';
		if (monday === addDays(now, 7)) return 'Next week';
		if (monday === addDays(now, -7)) return 'Last week';
		return `Week of ${shortDate(monday)}`;
	}

	// The week's dates, then the cycles its sessions come from.
	function weekNote(w: (typeof weeks)[number]) {
		const names = new Set(w.dates.flatMap((g) => g.days.flatMap((d) => (d.cycle ? [cycleNames.get(d.cycle) ?? 'Cycle'] : []))));
		return [`${shortDate(w.monday)} – ${shortDate(addDays(w.monday, 6))}`, ...names].join(' · ');
	}

	function rowNote(d: ScheduledDay) {
		const n = templates.get(d.template)?.sections.length ?? 0;
		const sections = n === 1 ? '1 section' : n ? `${n} sections` : '';
		return [statusWords[d.status], d.cycle ? '' : d.repeat ? 'Repeats' : 'One-off', sections].filter(Boolean).join(' · ');
	}

	function when(c: { starts: string; ends: string }) {
		if (c.ends < data.today) return '';
		if (c.starts > data.today) {
			const n = daysBetween(data.today, c.starts);
			return n === 1 ? 'starts tomorrow' : `starts in ${n} days`;
		}
		const w = cycleWeek(c.starts, c.ends, data.today);
		return `week ${w.week} of ${w.of}`;
	}

	const cycleState = (c: { starts: string; ends: string }) =>
		c.ends < data.today ? 'past' : c.starts > data.today ? 'next' : 'now';
	// Running cycles first, then the next to start, then the latest to end.
	const ordered = $derived.by(() => {
		const rank = { now: 0, next: 1, past: 2 };
		return [...data.cycles].sort((a, b) => {
			const ra = rank[cycleState(a)];
			const rb = rank[cycleState(b)];
			if (ra !== rb) return ra - rb;
			return ra === 1 ? a.starts.localeCompare(b.starts) : b.starts.localeCompare(a.starts);
		});
	});
	// From lg a Cycles row picks its cycle for the pane; a running one comes first.
	let pickedCycle = $state('');
	const cycle = $derived(ordered.find((c) => c.id === pickedCycle) ?? ordered[0]);
</script>

<svelte:head><title>Plan</title></svelte:head>

{#snippet row(g: { date: string }, d: ScheduledDay, i: number)}
	{@const note = rowNote(d)}
		<span class="flex w-11 shrink-0 flex-col">
			{#if i === 0}
				{#if g.date === data.today}
					<span class="text-xs font-bold text-ink">Today</span>
				{:else}
					<span class="text-xs font-semibold text-ink-2">{weekday(g.date)}</span>
				{/if}
				<b class="text-xl leading-[1.05] font-bold">{formatDate(g.date, { day: 'numeric' })}</b>
			{/if}
		</span>
		<SessionIcon icon={d.template_icon} name={d.template_name} state={iconState(d, data.today)} size={36} />
		<span class="min-w-0 flex-1">
			<span class="block truncate text-[15px] font-bold {d.status === 'missed' || d.status === 'paused' ? 'text-ink-2' : ''}">{d.template_name}</span>
			{#if note}
				<span class="mt-0.5 block truncate text-xs font-semibold text-ink-2">{note}</span>
			{/if}
		</span>
{/snippet}

{#snippet views()}
	<Segmented
		label="Plan"
		replace
		items={[
			{ label: 'Schedule', href: '/plan', on: !cyclesView && !calendarView },
			{ label: 'Calendar', href: '/plan?view=calendar', on: calendarView },
			{ label: 'Cycles', href: '/plan?view=cycles', on: cyclesView }
		]}
	/>
{/snippet}

<div class="flex flex-col gap-3.5 px-4 pt-[calc(env(safe-area-inset-top)+0.75rem)] pb-8 lg:mx-auto lg:max-w-[1280px] lg:px-8 lg:pt-3">
	<header class="flex items-start justify-between gap-3">
		<div>
			<h1 class="page-title">Plan</h1>
			{#if data.today}
				<p class="mt-1 text-[15px] font-semibold text-ink-2">{fullDate(data.today)}</p>
			{/if}
		</div>
		<div class="flex items-center gap-3">
			<div class="hidden w-[380px] lg:block">{@render views()}</div>
			{#if !data.offline}
				{#if cyclesView}
					<a href="/plan/cycles/new" class="mt-1 flex size-11 items-center justify-center rounded-full bg-surface text-ink shadow-card-sm lg:mt-0 lg:h-12 lg:w-41 lg:gap-2 lg:shadow-card" aria-label="New cycle">
						<Icon name="plus" size="1.25rem" /><span class="hidden text-[15px] font-bold lg:inline" aria-hidden="true">New cycle</span>
					</a>
				{:else}
					<div class="mt-1 lg:mt-0">
						<Menu
							label="Add"
							look="bg-surface text-ink shadow-card-sm lg:h-12 lg:w-41 lg:gap-2 lg:shadow-card"
							items={[
								...(data.templates.length ? [{ label: 'A session', onclick: openAdd }] : []),
								{ label: 'Time off', onclick: () => openOff(null) }
							]}
						>
							<Icon name="plus" size="1.25rem" /><span class="hidden text-[15px] font-bold lg:inline" aria-hidden="true">Add</span>
						</Menu>
					</div>
				{/if}
			{/if}
		</div>
	</header>

	<div class="lg:hidden">{@render views()}</div>

	{#if data.offline}
		<p class="rounded-3xl bg-surface p-[18px] text-[15px] font-semibold text-ink-2 shadow-card">No signal. The plan needs one to load.</p>
	{:else if calendarView}
		<div class="flex flex-col gap-3.5 lg:mt-2 lg:grid lg:grid-cols-[minmax(0,1fr)_330px] lg:items-start lg:gap-[22px]">
			<section class="flex flex-col gap-2" aria-label="Calendar">
				<div class="flex h-11 items-center justify-between pl-1">
					<h2 class="text-xl font-bold">{months}</h2>
					<nav class="flex gap-1" aria-label="Weeks">
						{#each [[-28, 'chevron-left', 'Earlier weeks'], [28, 'chevron-right', 'Later weeks']] as const as [n, icon, label] (n)}
							<a
								href={calendarAt(addDays(data.from, n))}
								class="flex size-11 items-center justify-center rounded-full text-ink-2"
								aria-label={label}
								data-sveltekit-replacestate
								data-sveltekit-noscroll
							>
								<Icon name={icon} size="1.25rem" />
							</a>
						{/each}
					</nav>
				</div>
				<PlanCalendar from={data.from} today={data.today} days={data.days} timeOff={data.timeOff} {colors} {selected} onselect={(date) => (chosen = date)} onmove={dragged} />
			</section>
			<DayPane date={selected} today={data.today} days={picked} live={!!data.live} {day} {cycleNames} timeOff={shownOff} onoff={openOff} />
		</div>
	{:else if !cyclesView}
		<div class="flex flex-col gap-3.5 lg:mt-2 lg:grid lg:grid-cols-[minmax(0,1fr)_330px] lg:items-start lg:gap-[22px]">
			<div class="flex flex-col gap-3.5">
			{#each weeks as w (w.monday)}
				<section class="flex flex-col gap-2">
					<div class="flex items-baseline justify-between gap-3 px-1 pt-1.5">
						<h2 class="shrink-0 text-xl font-bold">{weekTitle(w.monday)}</h2>
						<span class="truncate text-xs font-semibold text-ink-2">{weekNote(w)}</span>
					</div>
					<div class="rounded-3xl bg-surface px-[18px] py-1 shadow-card">
						<ul>
							{#each w.dates as g (g.date)}
								{#if wide.current}
									<li class="list-row">
										<button
											type="button"
											class="-mx-2.5 flex w-[calc(100%+1.25rem)] flex-col rounded-2xl px-2.5 text-left {g.date === selected ? 'bg-well' : ''}"
											aria-pressed={g.date === selected}
											onclick={() => (chosen = g.date)}
										>
											{#each g.days as d, i (d.id)}
												<span class="list-row flex min-h-[58px] items-center gap-3.5 py-2">{@render row(g, d, i)}</span>
											{/each}
										</button>
									</li>
								{:else}
									{#each g.days as d, i (d.id)}
											<li class="list-row">
											{#if d.status === 'planned' || d.status === 'paused' || d.status === 'missed'}
												<details class="group">
													<summary class="flex min-h-[58px] cursor-pointer list-none items-center gap-3.5 py-2 [&::-webkit-details-marker]:hidden">
														{@render row(g, d, i)}
														{#if (d.status === 'planned' || d.status === 'paused') && d.local_date === data.today && !data.live}
															<button
																type="button"
																class="flex size-11 shrink-0 items-center justify-center rounded-full bg-tint text-on-tint shadow-tint disabled:opacity-50"
																aria-label="Start {d.template_name}"
																disabled={day.starting}
																onclick={(e) => {
																	e.preventDefault();
																	e.stopPropagation();
																	day.start(d);
																}}
															>
																<Icon name="play" size="1.125rem" />
															</button>
														{:else}
															<span class="shrink-0 text-ink-3 transition-transform group-open:rotate-90"><Icon name="chevron-right" size="1rem" /></span>
														{/if}
													</summary>
													<form
														class="flex flex-col gap-3 pb-4 pl-[108px]"
														onsubmit={(e) => {
															e.preventDefault();
															day.move(d, String(new FormData(e.currentTarget).get('date')));
														}}
													>
														<div class="flex gap-2">
															<div class="min-w-0 flex-1"><DateField value={d.local_date} name="date" label="New day" /></div>
															<button type="submit" class="h-12 shrink-0 rounded-full bg-well px-4 text-[15px] font-bold text-ink disabled:opacity-50" disabled={day.doing !== null}>
																{day.doing === `move:${d.id}` ? 'Moving…' : 'Move'}
															</button>
														</div>
														<FormError message={day.error.id === d.id ? day.error.message : ''} />
														<button type="button" class="h-11 self-start text-[15px] font-bold text-bad disabled:opacity-50" disabled={day.doing !== null} onclick={() => day.remove(d)}>
															{day.doing === `remove:${d.id}` ? 'Removing…' : 'Remove'}
														</button>
													</form>
												</details>
											{:else}
												<a href={d.status === 'done' ? `/history/${d.run}` : `/run/${d.run}`} class="flex min-h-[58px] items-center gap-3.5 py-2">
													{@render row(g, d, i)}
													<span class="shrink-0 text-ink-3"><Icon name="chevron-right" size="1rem" /></span>
												</a>
											{/if}
										</li>
									{/each}
								{/if}
							{/each}
						</ul>
					</div>
				</section>
			{:else}
				<section class="flex flex-col gap-3 rounded-3xl bg-surface p-5 shadow-card">
					<p class="text-xl font-bold tracking-tight">Nothing planned</p>
					<p class="text-[15px] font-semibold text-ink-2">Nothing in the next four weeks.</p>
					{#if data.templates.length}
						<button type="button" class="flex h-12 items-center justify-center rounded-full bg-tint text-[15px] font-bold text-on-tint shadow-tint" onclick={openAdd}>Add a session</button>
					{/if}
				</section>
			{/each}
			</div>
			{#if wide.current}
				<div class="sticky top-6">
					<DayPane date={selected} today={data.today} days={picked} live={!!data.live} {day} {cycleNames} timeOff={shownOff} onoff={openOff} />
				</div>
			{/if}
		</div>
	{:else}
		{#snippet cycleRow(c: (typeof ordered)[number])}
			{@const state = cycleState(c)}
			<span
				class="size-2.5 shrink-0 rounded-full {state === 'now'
					? 'bg-live shadow-[0_0_0_4px_var(--live-halo)]'
					: state === 'next'
						? 'shadow-[inset_0_0_0_1.5px_var(--ink-3)]'
						: 'bg-ink-3 opacity-50'}"
				aria-hidden="true"
			></span>
			<span class="min-w-0 flex-1">
				<span class="block truncate text-[15px] font-bold">{c.name}</span>
				<span class="mt-0.5 block truncate text-xs font-semibold text-ink-2">
					{shortDate(c.starts)} – {shortDate(c.ends)}{when(c) ? ` · ${when(c)}` : ''}
				</span>
			</span>
			{#if state === 'now'}
				<span class="shrink-0 rounded-xl bg-live px-2.5 py-[5px] text-xs font-bold text-on-live">Now</span>
			{:else if !wide.current}
				<span class="text-ink-3"><Icon name="chevron-right" size="1rem" /></span>
			{/if}
		{/snippet}
		<div class="flex flex-col gap-3.5 lg:mt-2 lg:grid lg:grid-cols-[minmax(0,1fr)_330px] lg:items-start lg:gap-[22px]">
			<div class="flex flex-col gap-3.5">
				{#if data.cycles.length}
					<CycleYear cycles={data.cycles} today={data.today} />
					<h2 class="px-1 text-xl font-bold">Cycles</h2>
				{/if}
				<section class="rounded-3xl bg-surface px-[18px] pt-2 pb-2 shadow-card">
					{#if data.cycles.length}
						<ul>
							{#each ordered as c (c.id)}
								<li class="list-row">
									{#if wide.current}
										<button
											type="button"
											class="-mx-2.5 flex min-h-[58px] w-[calc(100%+1.25rem)] items-center gap-3 rounded-2xl px-2.5 py-2 text-left {c.id === cycle?.id ? 'bg-well' : ''}"
											aria-pressed={c.id === cycle?.id}
											onclick={() => (pickedCycle = c.id)}
										>
											{@render cycleRow(c)}
										</button>
									{:else}
										<a href="/plan/cycles/{c.id}" class="flex min-h-[58px] items-center gap-3 py-2">{@render cycleRow(c)}</a>
									{/if}
								</li>
							{/each}
						</ul>
					{:else}
						<div class="flex flex-col gap-3 pt-1 pb-3">
							<p class="text-[15px] font-semibold text-ink-2">A cycle repeats a block of sessions over weeks.</p>
							<a href="/plan/cycles/new" class="flex h-12 items-center justify-center rounded-full bg-tint text-[15px] font-bold text-on-tint shadow-tint">New cycle</a>
						</div>
					{/if}
				</section>
			</div>
			{#if wide.current && cycle}
				<section class="sticky top-6 flex flex-col gap-2" aria-live="polite">
					<div class="flex h-11 items-center justify-between gap-3 px-1">
						<h2 class="truncate text-[15px] font-bold">{cycle.name}</h2>
						<span class="shrink-0 text-xs font-semibold text-ink-2">{shortDate(cycle.starts)} – {shortDate(cycle.ends)}</span>
					</div>
					<div class="flex flex-col gap-4 rounded-3xl bg-surface p-[18px] shadow-card">
						<div>
							<h3 class="text-xs font-semibold text-ink-2">Block · every {cycle.block_days} days</h3>
							<ol class="mt-2.5 flex flex-wrap items-center gap-1.5">
								{#each Array.from({ length: cycle.block_days }, (_, i) => i + 1) as n (n)}
									{@const t = templates.get(cycle.days.find((d) => d.day === n)?.template ?? '')}
									<li class="flex size-[26px] items-center justify-center" aria-label="Day {n}: {t?.name ?? 'Rest'}">
										{#if t}
											<SessionIcon icon={t.icon} name={t.name} size={26} />
										{:else}
											<span class="size-1 rounded-full bg-ink-3"></span>
										{/if}
									</li>
								{/each}
							</ol>
						</div>
						{#if cycleState(cycle) === 'now'}
							<div>
								<h3 class="text-xs font-semibold text-ink-2">This week</h3>
								<div class="-mx-4 -mt-1"><WeekStrip monday={mondayOf(data.today)} today={data.today} week={data.days.filter((d) => d.local_date >= mondayOf(data.today) && d.local_date <= addDays(mondayOf(data.today), 6))} /></div>
							</div>
						{/if}
						<a href="/plan/cycles/{cycle.id}" class="flex h-12 items-center justify-center rounded-full bg-well text-[15px] font-bold shadow-card-sm">Open cycle</a>
					</div>
				</section>
			{/if}
		</div>
	{/if}
</div>

{#if !data.offline}
	<AddSession bind:open={adding} templates={data.templates} days={data.days} timeOff={data.timeOff} today={data.today} day={calendarView || wide.current ? selected : data.today} />
	<TimeOffSheet bind:open={offOpen} editing={editingOff} today={data.today} day={calendarView || wide.current ? selected : data.today} />
{/if}

<DaySheets {day} repeats={data.repeats} />
