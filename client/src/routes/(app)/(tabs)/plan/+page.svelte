<script lang="ts">
	import { goto, invalidateAll } from '$app/navigation';
	import { page } from '$app/state';
	import { describe, request } from '$lib/api';
	import Button from '$lib/Button.svelte';
	import CycleYear from '$lib/CycleYear.svelte';
	import { addDays, cycleWeek, daysBetween, formatDate, mondayOf } from '$lib/dates';
	import FormError from '$lib/FormError.svelte';
	import Icon from '$lib/Icon.svelte';
	import SessionIcon, { iconState } from '$lib/SessionIcon.svelte';
	import type { ScheduledDay } from '$lib/plan';
	import { startRun } from '$lib/runState.svelte';

	let { data } = $props();

	const cyclesView = $derived(page.url.searchParams.get('view') === 'cycles');

	const labels: Record<string, string> = { local_date: 'That day', template: 'That session' };

	let adding = $state(false);
	let template = $state('');
	let date = $state('');
	let busy = $state(false);
	let error = $state('');

	function openAdd() {
		adding = true;
		template = data.templates[0]?.id ?? '';
		date = data.today;
		error = '';
	}

	async function add() {
		busy = true;
		error = '';
		try {
			await request('POST', '/api/v1/scheduled-sessions', { template, local_date: date });
			await invalidateAll();
			adding = false;
		} catch (e) {
			error = describe(e, (f) => labels[f] ?? f);
		} finally {
			busy = false;
		}
	}

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
		busy = true;
		rowError = { id: '', message: '' };
		try {
			await request('PUT', `/api/v1/scheduled-sessions/${d.id}`, { local_date: to });
			await invalidateAll();
		} catch (e) {
			rowError = { id: d.id, message: describe(e, (f) => labels[f] ?? f) };
		} finally {
			busy = false;
		}
	}

	async function remove(d: ScheduledDay) {
		const warning = d.cycle
			? ' It comes back if you change the dates, the block or the days of its cycle.'
			: '';
		if (!confirm(`Take ${d.template_name} off this day?${warning}`)) return;
		busy = true;
		rowError = { id: '', message: '' };
		try {
			await request('DELETE', `/api/v1/scheduled-sessions/${d.id}`);
			await invalidateAll();
		} catch (e) {
			rowError = { id: d.id, message: describe(e, (f) => labels[f] ?? f) };
		} finally {
			busy = false;
		}
	}

	const cycleNames = $derived(new Map(data.cycles.map((c) => [c.id, c.name])));
	const templates = $derived(new Map(data.templates.map((t) => [t.id, t])));

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
		return `Week of ${short(monday)}`;
	}

	// The week's dates, then the cycles its sessions come from.
	function weekNote(w: (typeof weeks)[number]) {
		const names = new Set(w.dates.flatMap((g) => g.days.flatMap((d) => (d.cycle ? [cycleNames.get(d.cycle) ?? 'Cycle'] : []))));
		return [`${short(w.monday)} – ${short(addDays(w.monday, 6))}`, ...names].join(' · ');
	}

	function rowNote(d: ScheduledDay) {
		const n = templates.get(d.template)?.sections.length ?? 0;
		const sections = n === 1 ? '1 section' : n ? `${n} sections` : '';
		return [d.cycle ? '' : 'One-off', sections].filter(Boolean).join(' · ');
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

	const short = (date: string) => formatDate(date, { day: 'numeric', month: 'short' });

	const statuses = { done: 'Done', started: 'Started', missed: 'Missed', planned: '' };
	const tags = {
		done: 'bg-well text-ink-2',
		started: 'bg-ink text-ground',
		missed: 'text-ink-2 shadow-[inset_0_0_0_1.5px_var(--well)]',
		planned: ''
	};
</script>

<svelte:head><title>Plan</title></svelte:head>

<div class="flex flex-col gap-3.5 px-4 pt-[calc(env(safe-area-inset-top)+0.75rem)]">
	<header>
		<h1 class="text-[32px] leading-[1.1] font-extrabold tracking-[-0.02em]">Plan</h1>
		{#if data.today}
			<p class="mt-1 text-[15px] font-semibold text-ink-2">{formatDate(data.today, { weekday: 'long', day: 'numeric', month: 'long' })}</p>
		{/if}
	</header>

	<nav class="grid grid-cols-2 gap-1 rounded-full bg-well p-1" aria-label="Plan">
		{#each [{ label: 'Schedule', href: '/plan', on: !cyclesView }, { label: 'Cycles', href: '/plan?view=cycles', on: cyclesView }] as v (v.label)}
			<a
				href={v.href}
				data-sveltekit-replacestate
				data-sveltekit-noscroll
				class="flex h-11 items-center justify-center rounded-full text-[15px] font-bold {v.on ? 'bg-surface text-ink shadow-card-sm' : 'text-ink-2'}"
				aria-current={v.on ? 'page' : undefined}
			>
				{v.label}
			</a>
		{/each}
	</nav>

	{#if data.offline}
		<p class="rounded-3xl bg-surface p-[18px] text-[15px] font-semibold text-ink-2 shadow-card">No signal. The plan needs one to load.</p>
	{:else if !cyclesView}
		{#each weeks as w (w.monday)}
			<section class="flex flex-col gap-2">
				<div class="flex items-baseline justify-between gap-3 px-1 pt-1.5">
					<h2 class="shrink-0 text-xl font-bold tracking-tight">{weekTitle(w.monday)}</h2>
					<span class="truncate text-xs font-semibold text-ink-2">{weekNote(w)}</span>
				</div>
				<div class="rounded-3xl bg-surface px-[18px] py-1 shadow-card">
					<ul>
						{#each w.dates as g (g.date)}
							{#each g.days as d, i (d.id)}
								{#snippet row()}
									<span class="flex w-11 shrink-0 flex-col">
										{#if i === 0}
											{#if g.date === data.today}
												<span class="text-xs font-bold text-ink">Today</span>
											{:else}
												<span class="text-xs font-semibold text-ink-2">{formatDate(g.date, { weekday: 'short' })}</span>
											{/if}
											<b class="text-xl leading-[1.05] font-bold">{formatDate(g.date, { day: 'numeric' })}</b>
										{/if}
									</span>
									<SessionIcon icon={d.template_icon} name={d.template_name} state={iconState(d, data.today)} size={36} />
									<span class="min-w-0 flex-1">
										<span class="block truncate text-[15px] font-bold {d.status === 'missed' ? 'text-ink-2' : ''}">{d.template_name}</span>
										{#if rowNote(d)}
											<span class="mt-0.5 block truncate text-xs font-semibold text-ink-2">{rowNote(d)}</span>
										{/if}
									</span>
									{#if statuses[d.status]}
										<span class="shrink-0 rounded-xl px-2.5 py-[5px] text-xs font-bold {tags[d.status]}">{statuses[d.status]}</span>
									{/if}
								{/snippet}
								<li class="[&:not(:first-child)]:shadow-[inset_0_1px_0_var(--line)]">
									{#if d.status === 'planned' || d.status === 'missed'}
										<details class="group">
											<summary class="flex min-h-[58px] cursor-pointer list-none items-center gap-3.5 py-2 [&::-webkit-details-marker]:hidden">
												{@render row()}
												{#if d.status === 'planned' && d.local_date === data.today && !data.live}
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
													<input class="input h-12 px-4 min-w-0 flex-1" type="date" name="date" value={d.local_date} required aria-label="New day" />
													<button type="submit" class="h-12 shrink-0 rounded-full bg-well px-4 text-[15px] font-bold text-ink disabled:opacity-50" disabled={busy}>
														Move
													</button>
												</div>
												<FormError message={rowError.id === d.id ? rowError.message : ''} />
												<button type="button" class="h-11 self-start text-[15px] font-bold text-bad disabled:opacity-50" disabled={busy} onclick={() => remove(d)}>
													Remove
												</button>
											</form>
										</details>
									{:else}
										<div class="flex min-h-[58px] items-center gap-3.5 py-2">{@render row()}</div>
									{/if}
								</li>
							{/each}
						{/each}
					</ul>
				</div>
			</section>
		{:else}
			<p class="rounded-3xl bg-surface p-[18px] text-[15px] font-semibold text-ink-2 shadow-card">Nothing planned for the next four weeks.</p>
		{/each}

		{#if adding}
			<form
				class="flex flex-col gap-3 rounded-3xl bg-surface p-[18px] shadow-card"
				onsubmit={(e) => {
					e.preventDefault();
					add();
				}}
			>
				<label class="flex flex-col gap-1.5 text-xs font-semibold text-ink-2">
					Session
					<select class="input h-12 px-4" bind:value={template} required>
						{#each data.templates as t (t.id)}
							<option value={t.id}>{t.name}</option>
						{/each}
					</select>
				</label>
				<label class="flex flex-col gap-1.5 text-xs font-semibold text-ink-2">
					Day
					<input class="input h-12 px-4" type="date" bind:value={date} required />
				</label>
				<FormError message={error} />
				<div class="grid grid-cols-2 items-center gap-2">
					<Button variant="secondary" onclick={() => (adding = false)}>Cancel</Button>
					<Button type="submit" disabled={busy}>Add</Button>
				</div>
			</form>
		{:else}
			<Button variant="secondary" onclick={openAdd}>Add a session</Button>
		{/if}
	{:else}
		{#if data.cycles.length}
			<CycleYear cycles={data.cycles} today={data.today} />
			<h2 class="px-1 text-[15px] font-bold">Cycles</h2>
		{/if}
		<section class="rounded-3xl bg-surface px-[18px] pt-2 pb-2 shadow-card">
			{#if data.cycles.length}
				<ul>
					{#each ordered as c (c.id)}
						{@const state = cycleState(c)}
						<li class="[&:not(:first-child)]:shadow-[inset_0_1px_0_var(--line)]">
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
										{short(c.starts)} – {short(c.ends)}{when(c) ? ` · ${when(c)}` : ''}
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
				<p class="pt-1 pb-3 text-[15px] font-semibold text-ink-2">A cycle repeats a block of sessions over weeks.</p>
			{/if}
		</section>
		<Button variant="secondary" href="/plan/cycles/new">New cycle</Button>
	{/if}
</div>
