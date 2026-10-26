<script lang="ts">
	import AddSession from '$lib/AddSession.svelte';
	import Segmented from '$lib/Segmented.svelte';
	import { goto, invalidateAll } from '$app/navigation';
	import { page } from '$app/state';
	import { describe, request } from '$lib/api';
	import Button from '$lib/Button.svelte';
	import CycleYear from '$lib/CycleYear.svelte';
	import { addDays, cycleWeek, dayDate, daysBetween, formatDate, fullDate, mondayOf, shortDate, weekday } from '$lib/dates';
	import DateField from '$lib/DateField.svelte';
	import FormError from '$lib/FormError.svelte';
	import Icon from '$lib/Icon.svelte';
	import Menu from '$lib/Menu.svelte';
	import PlanCalendar from '$lib/PlanCalendar.svelte';
	import SessionIcon, { iconState } from '$lib/SessionIcon.svelte';
	import type { ScheduledDay, TimeOff } from '$lib/plan';
	import { ruleText } from '$lib/repeat';
	import { startRun } from '$lib/runState.svelte';
	import Sheet from '$lib/Sheet.svelte';
	import TimeOffSheet from '$lib/TimeOffSheet.svelte';

	let { data } = $props();

	const view = $derived(page.url.searchParams.get('view'));
	const cyclesView = $derived(view === 'cycles');
	const calendarView = $derived(view === 'calendar');

	const labels: Record<string, string> = { local_date: 'That day', template: 'That session' };

	let adding = $state(false);
	const openAdd = () => (adding = true);
	let offOpen = $state(false);
	let editingOff = $state<TimeOff | null>(null);
	function openOff(o: TimeOff | null) {
		editingOff = o;
		offOpen = true;
	}
	const offIcons = { trip: 'plane', injury: 'bandage', illness: 'heart-pulse', other: 'calendar' } as const;
	const offKinds = { trip: 'Trip', injury: 'Injury', illness: 'Illness', other: 'Other' };
	// The time off from the first week shown on.
	const shownOff = $derived(data.timeOff.filter((o) => !o.ends || o.ends >= data.from));
	// What is waiting: a row's move or remove.
	let doing = $state<string | null>(null);

	let rowError = $state({ id: '', message: '' });
	let starting = $state(false);

	async function start(d: ScheduledDay) {
		starting = true;
		rowError = { id: '', message: '' };
		try {
			const run = await startRun({ scheduled: d.id });
			await goto(`/run/${run.id}`);
		} catch (e) {
			rowError = { id: d.id, message: describe(e, (f) => labels[f] ?? f) };
			// The run can start even when the answer is lost; the reload shows it.
			await invalidateAll();
		} finally {
			starting = false;
		}
	}

	async function move(d: ScheduledDay, to: string) {
		doing = `move:${d.id}`;
		rowError = { id: '', message: '' };
		try {
			await request('PUT', `/api/v1/scheduled-sessions/${d.id}`, { local_date: to });
			await invalidateAll();
			moveOpen = false;
		} catch (e) {
			rowError = { id: d.id, message: describe(e, (f) => labels[f] ?? f) };
		} finally {
			doing = null;
		}
	}

	// A repeat's day asks whether its later days go too.
	let ending = $state<ScheduledDay | null>(null);
	let endOpen = $state(false);
	const endingRepeat = $derived(data.repeats.find((r) => r.id === ending?.repeat));

	async function remove(d: ScheduledDay, later = false) {
		if (d.repeat && !endOpen) {
			ending = d;
			endOpen = true;
			rowError = { id: '', message: '' };
			return;
		}
		const warning = d.cycle ? ' It comes back if you change the dates, the block or the days of its cycle.' : '';
		if (!d.repeat && !confirm(`Take ${d.template_name} off this day?${warning}`)) return;
		doing = `remove:${d.id}`;
		rowError = { id: '', message: '' };
		try {
			if (later) await request('POST', `/api/v1/repeats/${d.repeat}/end`, { from: d.local_date });
			else await request('DELETE', `/api/v1/scheduled-sessions/${d.id}`);
			await invalidateAll();
			endOpen = false;
		} catch (e) {
			rowError = { id: d.id, message: describe(e, (f) => labels[f] ?? f) };
		} finally {
			doing = null;
		}
	}

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
	const months = $derived.by(() => {
		const a = formatDate(data.from, { month: 'long' });
		const b = formatDate(last, { month: 'long' });
		return a === b ? a : `${a} – ${b}`;
	});
	const calendarAt = (from: string) => `/plan?view=calendar&from=${from}`;

	let moving = $state<ScheduledDay | null>(null);
	let moveOpen = $state(false);
	let moveTo = $state('');
	function openMove(d: ScheduledDay) {
		moving = d;
		moveOpen = true;
		moveTo = d.local_date;
		rowError = { id: '', message: '' };
	}

	const source = (d: ScheduledDay) =>
		[statuses[d.status], d.cycle ? (cycleNames.get(d.cycle) ?? 'Cycle') : d.repeat ? 'Repeats' : 'One-off'].filter(Boolean).join(' · ');

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
		return [statuses[d.status], d.cycle ? '' : d.repeat ? 'Repeats' : 'One-off', sections].filter(Boolean).join(' · ');
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

	const statuses = { done: 'Done', started: 'Started', paused: 'Paused', missed: 'Missed', planned: '' };
</script>

<svelte:head><title>Plan</title></svelte:head>

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

<div class="flex flex-col gap-3.5 px-4 pt-[calc(env(safe-area-inset-top)+0.75rem)] pb-8 lg:mx-auto lg:px-8 lg:pt-3 {calendarView ? 'lg:max-w-[1280px]' : 'lg:max-w-[760px]'}">
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
					<a href="/plan/cycles/new" class="mt-1 flex size-11 items-center justify-center rounded-full bg-surface text-ink shadow-card-sm lg:mt-0 lg:h-12 lg:w-auto lg:gap-2 lg:px-5 lg:shadow-card" aria-label="New cycle">
						<Icon name="plus" size="1.25rem" /><span class="hidden text-[15px] font-bold lg:inline" aria-hidden="true">New cycle</span>
					</a>
				{:else}
					<div class="mt-1 lg:mt-0">
						<Menu
							label="Add"
							look="bg-surface text-ink shadow-card-sm lg:h-12 lg:w-auto lg:gap-2 lg:px-5 lg:shadow-card"
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
				<PlanCalendar from={data.from} today={data.today} days={data.days} timeOff={data.timeOff} {colors} {selected} onselect={(date) => (chosen = date)} />
			</section>
			<section class="flex flex-col gap-2" aria-live="polite">
				<div class="flex h-11 items-center justify-between px-1">
					<h2 class="text-[15px] font-bold">{selected === data.today ? 'Today' : formatDate(selected, { weekday: 'long' })}</h2>
					<span class="text-xs font-semibold text-ink-2">{dayDate(selected)}</span>
				</div>
				<div class="rounded-3xl bg-surface px-4 py-0.5 shadow-card">
					{#if picked.length}
						<ul>
							{#each picked as d (d.id)}
								<li class="list-row flex min-h-16 items-center gap-3 py-2">
									<SessionIcon icon={d.template_icon} name={d.template_name} state={iconState(d, data.today)} size={36} />
									<span class="min-w-0 flex-1">
										<span class="block truncate text-[15px] font-bold {d.status === 'missed' || d.status === 'paused' ? 'text-ink-2' : ''}">{d.template_name}</span>
										<span class="mt-0.5 block truncate text-xs font-semibold text-ink-2">{source(d)}</span>
									</span>
									{#if d.run}
										<a href={d.status === 'done' ? `/history/${d.run}` : `/run/${d.run}`} class="flex size-11 shrink-0 items-center justify-center text-ink-3" aria-label="Open {d.template_name}">
											<Icon name="chevron-right" size="1rem" />
										</a>
									{:else}
										{#if (d.status === 'planned' || d.status === 'paused') && d.local_date === data.today && !data.live}
											<button
												type="button"
												class="flex size-11 shrink-0 items-center justify-center rounded-full bg-tint text-on-tint shadow-tint disabled:opacity-50"
												aria-label="Start {d.template_name}"
												disabled={starting}
												onclick={() => start(d)}
											>
												<Icon name="play" size="1.125rem" />
											</button>
										{/if}
										<Menu
											label="More for {d.template_name}"
											look="text-ink-2"
											items={[
												{ label: 'Move to another day', onclick: () => openMove(d) },
												{ label: 'Remove', danger: true, onclick: () => remove(d) }
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
				<FormError message={picked.some((d) => d.id === rowError.id) && !moveOpen && !endOpen ? rowError.message : ''} />
				{#if shownOff.length}
					<h2 class="flex h-11 items-center px-1 pt-2 text-[15px] font-bold">Time off</h2>
					<ul class="rounded-3xl bg-surface px-4 py-0.5 shadow-card">
						{#each shownOff as o (o.id)}
							<li class="list-row">
								<button type="button" class="flex min-h-16 w-full items-center gap-3 py-2 text-left" onclick={() => openOff(o)}>
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
		</div>
	{:else if !cyclesView}
		{#each weeks as w (w.monday)}
			<section class="flex flex-col gap-2">
				<div class="flex items-baseline justify-between gap-3 px-1 pt-1.5">
					<h2 class="shrink-0 text-xl font-bold">{weekTitle(w.monday)}</h2>
					<span class="truncate text-xs font-semibold text-ink-2">{weekNote(w)}</span>
				</div>
				<div class="rounded-3xl bg-surface px-[18px] py-1 shadow-card">
					<ul>
						{#each w.dates as g (g.date)}
							{#each g.days as d, i (d.id)}
								{@const note = rowNote(d)}
								{#snippet row()}
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
								<li class="list-row">
									{#if d.status === 'planned' || d.status === 'paused' || d.status === 'missed'}
										<details class="group">
											<summary class="flex min-h-[58px] cursor-pointer list-none items-center gap-3.5 py-2 [&::-webkit-details-marker]:hidden">
												{@render row()}
												{#if (d.status === 'planned' || d.status === 'paused') && d.local_date === data.today && !data.live}
													<button
														type="button"
														class="flex size-11 shrink-0 items-center justify-center rounded-full bg-tint text-on-tint shadow-tint disabled:opacity-50"
														aria-label="Start {d.template_name}"
														disabled={starting}
														onclick={(e) => {
															e.preventDefault();
															e.stopPropagation();
															start(d);
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
													move(d, String(new FormData(e.currentTarget).get('date')));
												}}
											>
												<div class="flex gap-2">
													<div class="min-w-0 flex-1"><DateField value={d.local_date} name="date" label="New day" /></div>
													<button type="submit" class="h-12 shrink-0 rounded-full bg-well px-4 text-[15px] font-bold text-ink disabled:opacity-50" disabled={doing !== null}>
														{doing === `move:${d.id}` ? 'Moving…' : 'Move'}
													</button>
												</div>
												<FormError message={rowError.id === d.id ? rowError.message : ''} />
												<button type="button" class="h-11 self-start text-[15px] font-bold text-bad disabled:opacity-50" disabled={doing !== null} onclick={() => remove(d)}>
													{doing === `remove:${d.id}` ? 'Removing…' : 'Remove'}
												</button>
											</form>
										</details>
									{:else}
										<a href={d.status === 'done' ? `/history/${d.run}` : `/run/${d.run}`} class="flex min-h-[58px] items-center gap-3.5 py-2">
											{@render row()}
											<span class="shrink-0 text-ink-3"><Icon name="chevron-right" size="1rem" /></span>
										</a>
									{/if}
								</li>
							{/each}
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

	{:else}
		{#if data.cycles.length}
			<CycleYear cycles={data.cycles} today={data.today} />
			<h2 class="px-1 text-xl font-bold">Cycles</h2>
		{/if}
		<section class="rounded-3xl bg-surface px-[18px] pt-2 pb-2 shadow-card">
			{#if data.cycles.length}
				<ul>
					{#each ordered as c (c.id)}
						{@const state = cycleState(c)}
						<li class="list-row">
							<a href="/plan/cycles/{c.id}" class="flex min-h-[58px] items-center gap-3 py-2">
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
								{:else}
									<span class="text-ink-3"><Icon name="chevron-right" size="1rem" /></span>
								{/if}
							</a>
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
	{/if}
</div>

{#if !data.offline}
	<AddSession bind:open={adding} templates={data.templates} days={data.days} timeOff={data.timeOff} today={data.today} day={calendarView ? selected : data.today} />
	<TimeOffSheet bind:open={offOpen} editing={editingOff} today={data.today} day={calendarView ? selected : data.today} />
{/if}

<Sheet bind:open={endOpen} title="Remove {ending?.template_name ?? ''}?">
	{#if ending}
		{@const d = ending}
		<div class="flex flex-col gap-3.5">
			{#if endingRepeat}
				<p class="px-1 text-[15px] leading-normal font-medium text-ink-2">It repeats {ruleText(endingRepeat)}.</p>
			{/if}
			<FormError message={rowError.id === d.id ? rowError.message : ''} />
			<div class="flex flex-col gap-2">
				<Button variant="secondary" disabled={doing !== null} onclick={() => remove(d)}>Only {formatDate(d.local_date, { weekday: 'long' })}, {shortDate(d.local_date)}</Button>
				<Button variant="danger" disabled={doing !== null} onclick={() => remove(d, true)}>This and later ones</Button>
			</div>
		</div>
	{/if}
</Sheet>

<Sheet bind:open={moveOpen} title="Move {moving?.template_name ?? ''}">
	{#if moving}
		{@const d = moving}
		<form
			class="flex flex-col gap-3.5"
			onsubmit={(e) => {
				e.preventDefault();
				move(d, moveTo);
			}}
		>
			<label class="flex flex-col gap-1.5 rounded-3xl bg-surface p-[18px] text-xs font-semibold text-ink-2 shadow-card">
				New day
				<DateField bind:value={moveTo} label="New day" />
			</label>
			<FormError message={rowError.id === d.id ? rowError.message : ''} />
			<div class="grid grid-cols-2 items-center gap-2">
				<Button variant="secondary" onclick={() => (moveOpen = false)}>Cancel</Button>
				<Button type="submit" disabled={doing !== null}>{doing === `move:${d.id}` ? 'Moving…' : 'Move'}</Button>
			</div>
		</form>
	{/if}
</Sheet>
