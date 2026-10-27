<script lang="ts">
	import { describe, request } from './api';
	import { dayDate, timeOfDay } from './dates';
	import DurationInput from './DurationInput.svelte';
	import FormBar from './FormBar.svelte';
	import { isStale, secondsSince, type Run } from './run';
	import { openRun } from './runState.svelte';
	import Segmented from './Segmented.svelte';
	import Select from './Select.svelte';
	import { guardUnsaved } from './unsaved';

	// The journal of the open run, which also finishes a running one. saved
	// runs once the server has it, before the run is closed here.
	let {
		cancel,
		saved,
		heading = 'h1'
	}: { cancel: string; saved: (run: Run) => Promise<unknown>; heading?: 'h1' | 'h2' } = $props();

	const run = $derived(openRun.run!);
	// A finished run comes here from History to edit its journal.
	const editing = !!openRun.run!.finished_at;
	// The clock of a run left open says days, so its length is asked for.
	const stale = isStale(openRun.run!);
	let length = $state(openRun.run!.elapsed_seconds);

	// The journal is optional, so each field starts from what the run holds.
	let sleep = $state(openRun.run!.sleep);
	let energy = $state(openRun.run!.energy);
	let rpe = $state(openRun.run!.rpe);
	let focus = $state(openRun.run!.focus ?? '');
	let setting = $state(openRun.run!.setting ?? '');
	let wentWell = $state(openRun.run!.went_well ?? '');
	let nextFocus = $state(openRun.run!.next_focus ?? '');
	let notes = $state(openRun.run!.notes ?? '');

	let busy = $state(false);
	let error = $state('');

	const fields = () => JSON.stringify([length, sleep, energy, rpe, focus, setting, wentWell, nextFocus, notes]);
	const clean = fields();
	let leaving = false;
	guardUnsaved(() => !leaving && fields() !== clean);

	const focuses = ['strength', 'endurance', 'technique', 'projects', 'general'];

	async function finish(e: SubmitEvent) {
		e.preventDefault();
		busy = true;
		error = '';
		const r = openRun.run!;
		if (!editing) openRun.logTimer();
		Object.assign(r, {
			sleep,
			energy,
			rpe,
			focus: focus || null,
			setting: setting || null,
			went_well: wentWell.trim() || null,
			next_focus: nextFocus.trim() || null,
			notes: notes.trim() || null,
			// A finished run keeps its length, even an unknown one.
			elapsed_seconds: editing ? r.elapsed_seconds : stale ? Math.round(length!) : (r.elapsed_seconds ?? secondsSince(r.started_at))
		});
		openRun.refused = '';
		openRun.saveBody();
		try {
			if (!(await openRun.settle())) {
				error = `Not saved yet. ${editing ? 'Saving' : 'Finish'} needs a signal. It is kept on this phone, so try again.`;
				return;
			}
			// The queue drops a refused write, so the journal would vanish without this.
			if (openRun.refused) {
				error = openRun.refused;
				return;
			}
			if (!editing) await request<Run>('POST', `/api/v1/runs/${r.id}/finish`);
			leaving = true;
			await saved($state.snapshot(r) as Run);
			openRun.forget(r);
		} catch (e) {
			error = describe(e, (f) => f);
		} finally {
			busy = false;
		}
	}
</script>

{#snippet rating(label: string, max: number, value: number | null, set: (v: number | null) => void)}
	<fieldset class="px-[18px] py-3.5 list-row">
		<legend class="float-left mb-2.5 w-full text-[15px] font-bold">{label}</legend>
		<!-- Five to a track, so 1–10 keeps each button 44 px wide on a phone. -->
		<div class="clear-left flex flex-col gap-2">
			{#each Array.from({ length: max / 5 }, (_, r) => r * 5) as start (start)}
				<Segmented
					label="{label} {start + 1}–{start + 5}"
					items={Array.from({ length: 5 }, (_, i) => start + i + 1).map((n) => ({
						label: `${n}`,
						on: value === n,
						onclick: () => set(value === n ? null : n)
					}))}
				/>
			{/each}
		</div>
	</fieldset>
{/snippet}

{#snippet note(label: string, value: string, set: (v: string) => void)}
	<label class="flex flex-col gap-2 px-[18px] py-3.5 list-row">
		<span class="text-[15px] font-bold">{label}</span>
		<textarea
			class="input min-h-20"
			{value}
			oninput={(e) => set(e.currentTarget.value)}
		></textarea>
	</label>
{/snippet}

<form class="flex flex-col gap-3 px-4 pb-[calc(env(safe-area-inset-bottom)+2rem)]" onsubmit={finish}>
	<FormBar
		title="Session journal"
		{cancel}
		{heading}
		saveLabel={editing ? 'Save' : 'Finish'}
		busyLabel={editing ? 'Saving…' : 'Finishing…'}
		{busy}
		disabled={stale && !((length ?? 0) >= 1)}
		{error}
		class="-mx-4"
	/>

	{#if stale}
		<div class="flex flex-col gap-1 rounded-3xl bg-surface p-[18px] shadow-card">
			<p class="text-[15px] font-bold">How long was it?</p>
			<p class="text-xs font-semibold text-ink-2">Started {dayDate(run.local_date)} at {timeOfDay(run.started_at, run.timezone)}.</p>
			<DurationInput id="length" bind:seconds={length} />
			{#if !((length ?? 0) >= 1)}<p class="mt-1 text-xs font-semibold text-ink-2">Finish needs a length.</p>{/if}
		</div>
		<p class="px-1 text-[15px] font-semibold text-ink-2">The rest is optional.</p>
	{:else}
		<p class="px-1 text-[15px] font-semibold text-ink-2">All optional. It helps to read it before the next session.</p>
	{/if}

	<div class="rounded-3xl bg-surface shadow-card">
		{@render rating('Sleep', 5, sleep, (v) => (sleep = v))}
		{@render rating('Energy before', 5, energy, (v) => (energy = v))}
		{@render rating('How hard it felt', 10, rpe, (v) => (rpe = v))}
	</div>

	<div class="grid grid-cols-2 gap-3 rounded-3xl bg-surface p-[18px] shadow-card">
		<label class="flex flex-col gap-1.5 text-xs font-semibold text-ink-2">
			Focus
			<Select bind:value={focus} label="Focus" options={[{ value: '', label: '–' }, ...focuses.map((f) => ({ value: f, label: f[0].toUpperCase() + f.slice(1) }))]} />
		</label>
		<label class="flex flex-col gap-1.5 text-xs font-semibold text-ink-2">
			Where
			<Select
				bind:value={setting}
				label="Where"
				options={[
					{ value: '', label: '–' },
					{ value: 'indoor', label: 'Indoor' },
					{ value: 'outdoor', label: 'Outdoor' }
				]}
			/>
		</label>
	</div>

	<div class="rounded-3xl bg-surface shadow-card">
		{@render note('What went well', wentWell, (v) => (wentWell = v))}
		{@render note('Next time, focus on', nextFocus, (v) => (nextFocus = v))}
		{@render note('Notes', notes, (v) => (notes = v))}
	</div>
</form>
