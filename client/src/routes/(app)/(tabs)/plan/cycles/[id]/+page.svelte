<script lang="ts">
	import { untrack } from 'svelte';
	import { MediaQuery } from 'svelte/reactivity';
	import { goto, invalidateAll } from '$app/navigation';
	import { page } from '$app/state';
	import { describe, request } from '$lib/api';
	import BlockDays from '$lib/BlockDays.svelte';
	import BlockLength from '$lib/BlockLength.svelte';
	import CycleCalendar from '$lib/CycleCalendar.svelte';
	import Button from '$lib/Button.svelte';
	import { addDays, cycleWeek, daysBetween, formatDate, gridWeeks, shortDate, weekday } from '$lib/dates';
	import { DayActions } from '$lib/dayActions.svelte';
	import DayPane from '$lib/DayPane.svelte';
	import DaySheets from '$lib/DaySheets.svelte';
	import DateField from '$lib/DateField.svelte';
	import FormError from '$lib/FormError.svelte';
	import GoalList from '$lib/GoalList.svelte';
	import GoalSheet from '$lib/GoalSheet.svelte';
	import Icon from '$lib/Icon.svelte';
	import Menu from '$lib/Menu.svelte';
	import NavBar from '$lib/NavBar.svelte';
	import PlanCalendar from '$lib/PlanCalendar.svelte';
	import { lastEnd, saveCycle, type Cycle, type ScheduledDay, type TimeOff } from '$lib/plan';
	import Section from '$lib/Section.svelte';
	import Sheet from '$lib/Sheet.svelte';
	import TimeOffSheet from '$lib/TimeOffSheet.svelte';
	import Toast from '$lib/Toast.svelte';
	import WeekStrip from '$lib/WeekStrip.svelte';

	let { data } = $props();

	// From lg the cycle shows as Plan's calendar, with the picked day beside it.
	const wide = new MediaQuery('min-width: 64rem');
	const day = new DayActions();
	const cycleNames = $derived(new Map(Object.entries(data.cycleNames)));
	const colors = $derived(new Map(data.templates.map((t) => [t.id, t.color])));
	let offOpen = $state(false);
	let editingOff = $state<TimeOff | null>(null);

	// Up to eight weeks show at once; a longer cycle shows six at a time, from
	// the six that hold today.
	const allWeeks = $derived(gridWeeks(data.cycle.starts, data.cycle.ends));
	const pages = $derived(allWeeks.length > 8 ? Math.ceil(allWeeks.length / 6) : 1);
	let turned = $state<number | null>(null);
	const at = $derived.by(() => {
		if (turned !== null) return Math.min(turned, pages - 1);
		if (data.today > data.cycle.ends) return pages - 1;
		return Math.max(0, Math.floor(allWeeks.findIndex((w) => w[0] <= data.today && data.today <= w[6]) / 6));
	});
	const shown = $derived(pages > 1 ? allWeeks.slice(at * 6, at * 6 + 6) : allWeeks);
	const shownFrom = $derived(shown[0][0] < data.cycle.starts ? data.cycle.starts : shown[0][0]);
	const shownTo = $derived(shown.at(-1)![6] > data.cycle.ends ? data.cycle.ends : shown.at(-1)![6]);
	const months = $derived.by(() => {
		const a = formatDate(shownFrom, { month: 'long' });
		const b = formatDate(shownTo, { month: 'long' });
		return a === b ? a : `${a} – ${b}`;
	});

	let chosen = $state('');
	const inView = (date: string) => date >= shownFrom && date <= shownTo;
	const selected = $derived(inView(chosen) ? chosen : inView(data.today) ? data.today : shownFrom);

	// A refused drag leaves the session where it was, so its day shows why.
	async function dragged(d: ScheduledDay, to: string) {
		try {
			await move(d, to);
		} catch (e) {
			day.error = { id: d.id, message: e instanceof Error ? e.message : String(e) };
			chosen = d.local_date;
		}
	}

	let error = $state('');

	const when = $derived.by(() => {
		const c = data.cycle;
		if (data.today > c.ends) return 'Ended';
		if (data.today < c.starts) {
			const n = daysBetween(data.today, c.starts);
			return n === 1 ? 'Starts tomorrow' : `Starts in ${n} days`;
		}
		const w = cycleWeek(c.starts, c.ends, data.today);
		return `Week ${w.week} of ${w.of}`;
	});

	type Editor = 'name' | 'dates' | 'block';

	// An editor works on a copy, and saves the whole cycle with its change.
	let editing = $state<Editor | null>(null);
	let open = $state(false);
	let draft = $state<Cycle>(untrack(() => structuredClone($state.snapshot(data.cycle))));
	let busy = $state(false);
	let editError = $state('');
	let leftOut = $state(untrack(() => page.state.leftOut ?? 0));

	const begun = $derived(data.cycle.starts <= data.today);
	// Day 1 of the block as the server will count it after the save.
	const blockFrom = $derived(
		draft.block_days !== data.cycle.block_days && data.today > data.cycle.starts && data.today <= draft.ends
			? data.today
			: data.cycle.block_from
	);
	const titles: Record<Editor, string> = { name: 'Name', dates: 'Dates', block: 'Block' };
	const fields: Record<string, string> = { name: 'The name', starts: 'The start', ends: 'The end', block_days: 'The block' };

	function edit(e: Editor) {
		draft = structuredClone($state.snapshot(data.cycle));
		editError = '';
		editing = e;
		open = true;
	}

	async function save() {
		busy = true;
		editError = '';
		const d = $state.snapshot(draft);
		const change: Record<Editor, Partial<Cycle>> = {
			name: { name: d.name },
			dates: { starts: d.starts, ends: d.ends },
			block: { block_days: d.block_days, days: d.days }
		};
		try {
			leftOut = (await send(change[editing!])).leftOut;
			open = false;
			saved();
		} catch (e) {
			editError = describe(e, (f) => (f.startsWith('days') ? 'A session' : (fields[f] ?? f)));
		} finally {
			busy = false;
		}
	}

	// Goals and notes save as they change, with no Save button.
	let goals = $state(untrack(() => structuredClone($state.snapshot(data.cycle.goals))));
	let notes = $state(untrack(() => data.cycle.notes ?? ''));
	let goalOpen = $state(false);
	let goalIndex = $state(-1);

	// Every save waits for the one before it, then builds the whole cycle from
	// what that save left, so no save can send another's old values.
	let saving: Promise<unknown> = Promise.resolve();
	function send(change: Partial<Cycle> = {}) {
		const next = saving.then(async () => {
			const body = { ...$state.snapshot(data.cycle), goals: $state.snapshot(goals), notes: notes.trim() || null, ...change };
			const result = await saveCycle(body);
			await invalidateAll();
			return result;
		});
		saving = next.catch(() => {});
		return next;
	}

	async function savePart() {
		error = '';
		try {
			await send();
			saved();
		} catch (e) {
			error = describe(e, (f) => (f.startsWith('goals') ? 'A goal' : f));
		}
	}

	// A refused move says why in the calendar's day panel. A move that lands
	// can be undone from the toast; any other save just says so.
	let toast = $state('');
	let last = $state<{ id: string; from: string } | null>(null);

	function saved() {
		last = null;
		toast = 'Saved';
	}

	async function move(d: ScheduledDay, to: string) {
		try {
			await request('PUT', `/api/v1/scheduled-sessions/${d.id}`, { local_date: to });
			last = { id: d.id, from: d.local_date };
			toast = `Moved to ${weekday(to)} ${Number(to.slice(8))}`;
		} catch (e) {
			throw new Error(describe(e, () => 'That day'));
		} finally {
			await invalidateAll();
		}
	}

	async function undo() {
		if (!last) return;
		const { id, from } = last;
		last = null;
		try {
			await request('PUT', `/api/v1/scheduled-sessions/${id}`, { local_date: from });
		} catch (e) {
			error = describe(e, () => 'That day');
		} finally {
			await invalidateAll();
		}
	}

	async function remove() {
		if (!confirm(`Delete ${data.cycle.name}? Its planned sessions go. The sessions you ran stay in History.`)) return;
		error = '';
		try {
			await request('DELETE', `/api/v1/cycles/${data.cycle.id}`);
			await goto('/plan?view=cycles');
		} catch (e) {
			error = describe(e, (f) => f);
		}
	}
</script>

<svelte:head><title>{data.cycle.name}</title></svelte:head>

<NavBar back={{ href: '/plan?view=cycles', label: 'Plan' }} wide>
	{#snippet actions()}
		<Menu
			items={[
				{ label: 'Edit name', onclick: () => edit('name') },
				{ label: 'Edit dates', onclick: () => edit('dates') },
				{ label: 'Edit block', onclick: () => edit('block') },
				{ label: 'Delete cycle', danger: true, onclick: remove }
			]}
		/>
	{/snippet}
</NavBar>

{#snippet goalsSection()}
	<Section title="Goals">
		{#snippet aside()}
			{#if goals.length}
				<span class="text-xs font-semibold text-ink-2">{goals.filter((g) => g.done).length} of {goals.length} done</span>
			{/if}
		{/snippet}
		<GoalList
			bind:goals
			onchange={savePart}
			onedit={(i) => {
				goalIndex = i;
				goalOpen = true;
			}}
		/>
	</Section>
{/snippet}

{#snippet notesSection()}
	<Section title="Notes">
		<textarea
			class="min-h-28 w-full resize-none rounded-3xl bg-surface px-[18px] py-3.5 text-[15px] font-semibold shadow-card [field-sizing:content] placeholder:text-ink-3 focus:shadow-[0_0_0_2px_var(--ink-3)] focus:outline-none"
			placeholder="Anything to remember about this cycle"
			aria-label="Notes"
			bind:value={notes}
			onblur={() => notes.trim() !== (data.cycle.notes ?? '') && savePart()}
		></textarea>
	</Section>
{/snippet}

<div class="flex flex-col gap-3.5 px-4 pt-1 pb-[calc(env(safe-area-inset-bottom)+5rem)] lg:mx-auto lg:w-full lg:max-w-[1280px] lg:px-8 lg:pb-8">
	<header class="px-1">
		<h1 class="page-title break-words">{data.cycle.name}</h1>
		<p class="mt-1 text-[15px] font-semibold text-ink-2">
			{shortDate(data.cycle.starts)} – {shortDate(data.cycle.ends)} · {when} · {data.cycle.block_days}-day block
		</p>
	</header>

	<FormError message={error} />
	{#if leftOut}
		<p class="rounded-3xl bg-surface p-[18px] text-[15px] font-semibold shadow-card">
			Saved. {leftOut === 1 ? '1 day' : `${leftOut} days`} already held that session, so the cycle left
			{leftOut === 1 ? 'it as it was' : 'them as they were'}.
		</p>
	{/if}

	{#if wide.current}
		<div class="mt-2 grid grid-cols-[minmax(0,1fr)_330px] items-start gap-[22px]">
			<section class="flex flex-col gap-2" aria-label="Calendar">
				<div class="flex h-11 items-center justify-between gap-3 pl-1">
					<h2 class="text-xl font-bold">{months}</h2>
					<div class="flex items-center gap-1">
						{#if data.today <= data.cycle.ends}
							<span class="hidden pr-2 text-xs font-semibold text-ink-2 xl:inline">Drag a session to move it</span>
						{/if}
						{#if pages > 1}
							<nav class="flex gap-1" aria-label="Weeks">
								<button type="button" class="flex size-11 items-center justify-center rounded-full text-ink-2 disabled:opacity-40" aria-label="Earlier weeks" disabled={at === 0} onclick={() => (turned = at - 1)}>
									<Icon name="chevron-left" size="1.25rem" />
								</button>
								<button type="button" class="flex size-11 items-center justify-center rounded-full text-ink-2 disabled:opacity-40" aria-label="Later weeks" disabled={at === pages - 1} onclick={() => (turned = at + 1)}>
									<Icon name="chevron-right" size="1.25rem" />
								</button>
							</nav>
						{/if}
					</div>
				</div>
				<PlanCalendar
					from={shown[0][0]}
					count={shown.length}
					range={data.cycle}
					today={data.today}
					days={data.days}
					timeOff={data.timeOff}
					{colors}
					{selected}
					onselect={(date) => (chosen = date)}
					onmove={data.today <= data.cycle.ends ? dragged : undefined}
				/>
			</section>
			<div class="flex flex-col gap-3.5">
				<DayPane
					date={selected}
					today={data.today}
					days={data.days.filter((d) => d.local_date === selected)}
					live={!!data.live}
					{day}
					{cycleNames}
					timeOff={data.timeOff}
					onoff={(o) => {
						editingOff = o;
						offOpen = true;
					}}
				/>
				{@render goalsSection()}
				{@render notesSection()}
			</div>
		</div>
	{:else}
		{@render goalsSection()}
		{@render notesSection()}
	{/if}

	{#if !wide.current && data.cycle.starts <= data.today && data.today <= data.cycle.ends}
		<Section title="This week">
			{#snippet aside()}
				<span class="text-xs font-semibold text-ink-2">{shortDate(data.monday)} – {shortDate(addDays(data.monday, 6))}</span>
			{/snippet}
			<div class="rounded-3xl bg-surface pb-3 shadow-card">
				<WeekStrip monday={data.monday} today={data.today} week={data.week} />
				<p class="mx-[18px] mt-3 truncate pt-3 text-[15px] font-semibold text-ink-2 shadow-[inset_0_1px_0_var(--line)]">
					<b class="text-ink">Today</b>
					{data.week
						.filter((d) => d.local_date === data.today)
						.map((d) => d.template_name)
						.join(' · ') || 'Rest day'}
				</p>
			</div>
		</Section>
	{/if}

	{#if !wide.current}
		<Section title="Calendar">
			{#snippet aside()}
				{#if data.today <= data.cycle.ends}
					<span class="text-xs font-semibold text-ink-2">Hold a session to move it</span>
				{/if}
			{/snippet}
			<CycleCalendar cycle={data.cycle} days={data.days} today={data.today} cycleNames={data.cycleNames} onmove={move} />
		</Section>
	{/if}
</div>

<DaySheets {day} repeats={data.repeats} />
<TimeOffSheet bind:open={offOpen} editing={editingOff} today={data.today} day={selected} />

<Sheet bind:open title={editing ? titles[editing] : ''} wide={editing === 'block'}>
	<form
		class="flex flex-col gap-3.5"
		onsubmit={(e) => {
			e.preventDefault();
			save();
		}}
	>
		{#if editing === 'block'}
			<BlockLength bind:value={draft.block_days} max={daysBetween(draft.starts, draft.ends) + 1} />
			<BlockDays bind:days={draft.days} blockDays={draft.block_days} from={blockFrom} templates={data.templates} />
			<p class="px-1 text-xs font-semibold text-ink-2">Planned days from today on are set again, so a session you moved goes back.</p>
		{:else}
			<div class="flex flex-col gap-3.5 rounded-3xl bg-surface p-[18px] shadow-card">
				{#if editing === 'name'}
					<label class="flex flex-col gap-1.5 text-xs font-semibold text-ink-2">
						Name
						<input class="input h-12 px-4" bind:value={draft.name} required maxlength="200" />
					</label>
				{:else if editing === 'dates'}
					<div class="grid grid-cols-2 gap-3">
						<label class="flex flex-col gap-1.5 text-xs font-semibold text-ink-2">
							Starts
							<DateField bind:value={draft.starts} label="Starts" disabled={begun} />
						</label>
						<label class="flex flex-col gap-1.5 text-xs font-semibold text-ink-2">
							Ends
							<DateField bind:value={draft.ends} label="Ends" min={draft.starts} max={lastEnd(draft.starts)} />
						</label>
					</div>
					{#if begun}
						<p class="text-xs font-semibold text-ink-2">The start stays: the cycle has begun.</p>
					{/if}
					<p class="text-xs font-semibold text-ink-2">Planned days from today on are set again, so a session you moved goes back.</p>
				{/if}
			</div>
		{/if}
		<FormError message={editError} />
		<div class="grid grid-cols-2 items-center gap-2">
			<Button variant="secondary" onclick={() => (open = false)}>Cancel</Button>
			<Button type="submit" disabled={busy}>{busy ? 'Saving…' : 'Save'}</Button>
		</div>
	</form>
</Sheet>

<GoalSheet
	bind:open={goalOpen}
	{goals}
	index={goalIndex}
	onsave={(next) => {
		goals = next;
		savePart();
	}}
/>

<Toast bind:message={toast} action={last ? 'Undo' : undefined} onaction={undo} duration={last ? 6000 : 2000} />
