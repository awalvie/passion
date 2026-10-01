<script lang="ts">
	import { untrack } from 'svelte';
	import { goto, invalidateAll } from '$app/navigation';
	import { page } from '$app/state';
	import { describe, request } from '$lib/api';
	import BlockDays from '$lib/BlockDays.svelte';
	import BlockLength from '$lib/BlockLength.svelte';
	import CycleCalendar from '$lib/CycleCalendar.svelte';
	import Button from '$lib/Button.svelte';
	import { addDays, cycleWeek, daysBetween, formatDate } from '$lib/dates';
	import DateField from '$lib/DateField.svelte';
	import FormError from '$lib/FormError.svelte';
	import GoalList from '$lib/GoalList.svelte';
	import GoalSheet from '$lib/GoalSheet.svelte';
	import Menu from '$lib/Menu.svelte';
	import NavBar from '$lib/NavBar.svelte';
	import { saveCycle, type Cycle, type ScheduledDay } from '$lib/plan';
	import Sheet from '$lib/Sheet.svelte';
	import Toast from '$lib/Toast.svelte';
	import WeekStrip from '$lib/WeekStrip.svelte';

	let { data } = $props();

	let error = $state('');

	const short = (date: string) => formatDate(date, { day: 'numeric', month: 'short' });

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
		try {
			const body = $state.snapshot(draft);
			body.days = body.days.filter((d) => d.day <= body.block_days);
			leftOut = (await saveCycle(body)).leftOut;
			open = false;
			saved();
			await invalidateAll();
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

	// Each save sends both from the page, one after another, so a quick
	// second change cannot send the first one's old value.
	let saving = Promise.resolve();
	function savePart() {
		saving = saving.then(async () => {
			error = '';
			try {
				const live = $state.snapshot({ goals });
				await saveCycle({ ...$state.snapshot(data.cycle), ...live, notes: notes.trim() || null });
				saved();
				await invalidateAll();
			} catch (e) {
				error = describe(e, (f) => (f.startsWith('goals') ? 'A goal' : f));
			}
		});
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
			toast = `${d.template_name} moved to ${formatDate(to, { weekday: 'short', day: 'numeric', month: 'short' })}`;
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

<NavBar back={{ href: '/plan?view=cycles', label: 'Plan' }}>
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

<div class="flex flex-col gap-3.5 px-4 pt-1 pb-[calc(env(safe-area-inset-bottom)+5rem)]">
	<header class="px-1">
		<h1 class="text-[32px] leading-[1.1] font-extrabold tracking-[-0.02em] break-words">{data.cycle.name}</h1>
		<p class="mt-1 text-[15px] font-semibold text-ink-2">
			{short(data.cycle.starts)} – {short(data.cycle.ends)} · {when} · {data.cycle.block_days}-day block
		</p>
	</header>

	<FormError message={error} />
	{#if leftOut}
		<p class="rounded-3xl bg-surface p-[18px] text-[15px] font-semibold shadow-card">
			Saved. {leftOut === 1 ? '1 day' : `${leftOut} days`} already held that session, so the cycle left
			{leftOut === 1 ? 'it as it was' : 'them as they were'}.
		</p>
	{/if}

	<section class="flex flex-col gap-2">
		<div class="flex items-baseline justify-between px-1">
			<h2 class="text-[15px] font-bold">Goals</h2>
			{#if goals.length}
				<span class="text-xs font-semibold text-ink-2">{goals.filter((g) => g.done).length} of {goals.length} done</span>
			{/if}
		</div>
		<GoalList
			bind:goals
			onchange={savePart}
			onedit={(i) => {
				goalIndex = i;
				goalOpen = true;
			}}
		/>
	</section>

	<section class="flex flex-col gap-2">
		<h2 class="px-1 text-[15px] font-bold">Notes</h2>
		<textarea
			class="min-h-28 w-full resize-none rounded-3xl bg-surface px-[18px] py-3.5 text-[15px] font-semibold shadow-card [field-sizing:content] placeholder:text-ink-3 focus:shadow-[0_0_0_2px_var(--ink-3)] focus:outline-none"
			placeholder="Anything to remember about this cycle"
			aria-label="Notes"
			bind:value={notes}
			onblur={() => notes.trim() !== (data.cycle.notes ?? '') && savePart()}
		></textarea>
	</section>

	<section class="flex flex-col gap-2">
		<div class="flex items-baseline justify-between px-1">
			<h2 class="text-[15px] font-bold">This week</h2>
			<span class="text-xs font-semibold text-ink-2">{short(data.monday)} – {short(addDays(data.monday, 6))}</span>
		</div>
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
	</section>

	<section class="flex flex-col gap-2">
		<div class="flex items-baseline justify-between px-1">
			<h2 class="text-[15px] font-bold">Calendar</h2>
			{#if data.today <= data.cycle.ends}
				<span class="text-xs font-semibold text-ink-2">Hold a session to move it</span>
			{/if}
		</div>
		<CycleCalendar cycle={data.cycle} days={data.days} today={data.today} cycleNames={data.cycleNames} onmove={move} />
	</section>

</div>

<Sheet bind:open title={editing ? titles[editing] : ''}>
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
							<DateField bind:value={draft.ends} label="Ends" min={draft.starts} />
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
