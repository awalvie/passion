<script lang="ts">
	import { invalidateAll } from '$app/navigation';
	import { untrack } from 'svelte';
	import { describe, request } from './api';
	import Button from './Button.svelte';
	import Checkbox from './Checkbox.svelte';
	import DateField from './DateField.svelte';
	import { daysBetween, shortDate } from './dates';
	import DayPicker from './DayPicker.svelte';
	import FormError from './FormError.svelte';
	import { newId } from './id';
	import { notice } from './notice.svelte';
	import type { TimeOff } from './plan';
	import Segmented from './Segmented.svelte';
	import Sheet from './Sheet.svelte';
	import { plural } from './text';

	// Adds a time off, or changes the one in editing. The first tap on the
	// calendar sets the start, the next one the end.
	let {
		open = $bindable(false),
		editing,
		today,
		day
	}: { open?: boolean; editing: TimeOff | null; today: string; day: string } = $props();

	const kinds = [
		{ value: 'trip', label: 'Trip' },
		{ value: 'injury', label: 'Injury' },
		{ value: 'illness', label: 'Illness' },
		{ value: 'other', label: 'Other' }
	] as const;

	let kind = $state<TimeOff['kind']>('trip');
	let name = $state('');
	let starts = $state('');
	let ends = $state('');
	let noEnd = $state(false);
	let pauses = $state(true);
	let next = $state<'starts' | 'ends'>('starts');
	let saving = $state(false);
	let error = $state('');

	$effect(() => {
		if (open) untrack(reset);
	});

	function reset() {
		kind = editing?.kind ?? 'trip';
		name = editing?.name ?? '';
		starts = editing?.starts ?? day;
		ends = editing?.ends ?? editing?.starts ?? day;
		noEnd = !!editing && !editing.ends;
		pauses = editing?.pauses ?? true;
		next = 'starts';
		error = '';
	}

	function pick(date: string) {
		if (noEnd || next === 'starts' || date < starts) {
			starts = date;
			if (ends < date) ends = date;
			next = 'ends';
		} else {
			ends = date;
			next = 'starts';
		}
	}

	const last = $derived(noEnd ? null : ends);
	const labels: Record<string, string> = { kind: 'The kind', name: 'The name', starts: 'From', ends: 'Until' };

	async function save() {
		saving = true;
		error = '';
		try {
			await request('PUT', `/api/v1/time-off/${editing?.id ?? newId()}`, { kind, name, starts, ends: last, pauses });
			await invalidateAll();
			open = false;
			notice.text = 'Time off saved';
		} catch (e) {
			error = describe(e, (f) => labels[f] ?? f);
		} finally {
			saving = false;
		}
	}

	async function remove() {
		if (!editing || !confirm(`Delete ${editing.name}?`)) return;
		saving = true;
		error = '';
		try {
			await request('DELETE', `/api/v1/time-off/${editing.id}`);
			await invalidateAll();
			open = false;
			notice.text = 'Time off deleted';
		} catch (e) {
			error = describe(e, (f) => labels[f] ?? f);
		} finally {
			saving = false;
		}
	}
</script>

<Sheet bind:open title="Time off" wide>
	<form
		class="flex flex-col gap-3.5"
		onsubmit={(e) => {
			e.preventDefault();
			save();
		}}
	>
		<div class="flex flex-col gap-3.5 lg:grid lg:grid-cols-[minmax(0,1fr)_324px] lg:items-start">
			<div class="flex flex-col gap-4 rounded-3xl bg-surface p-[18px] shadow-card">
				<div class="flex flex-col gap-1.5 text-xs font-semibold text-ink-2">
					Kind
					<Segmented label="Kind" items={kinds.map((k) => ({ label: k.label, on: kind === k.value, onclick: () => (kind = k.value) }))} />
				</div>
				<label class="flex flex-col gap-1.5 text-xs font-semibold text-ink-2">
					Name
					<input class="input h-12 px-4" bind:value={name} required maxlength="200" placeholder="Fontainebleau" />
				</label>
				<div class="grid grid-cols-2 gap-3">
					<label class="flex min-w-0 flex-col gap-1.5 text-xs font-semibold text-ink-2">
						From
						<DateField bind:value={starts} label="From" />
					</label>
					<label class="flex min-w-0 flex-col gap-1.5 text-xs font-semibold text-ink-2">
						Until
						<DateField bind:value={ends} label="Until" min={starts} disabled={noEnd} />
					</label>
				</div>
				<div class="flex flex-col">
					<label class="flex min-h-11 items-center gap-3 text-[15px] font-semibold text-ink">
						<Checkbox bind:checked={noEnd} />No end date yet
					</label>
					<label class="flex min-h-11 items-center gap-3 text-[15px] font-semibold text-ink">
						<Checkbox bind:checked={pauses} />Pause planned sessions
					</label>
				</div>
			</div>
			<div>
				<div class="rounded-3xl bg-surface px-2.5 pt-1.5 pb-2.5 shadow-card">
					{#key open}
						<DayPicker
							start={starts || day}
							{today}
							mark={(date) => (date === starts || date === last ? 'end' : '')}
							off={(date) => date >= starts && (!last || date <= last)}
							onpick={pick}
						/>
					{/key}
				</div>
				<p class="mt-3 px-1 text-[15px] leading-normal font-medium text-ink-2">
					{#if !last}
						<b class="font-bold text-ink">From {shortDate(starts)}</b>, with no end date yet.
					{:else if last === starts}
						<b class="font-bold text-ink">1 day</b>, {shortDate(starts)}.
					{:else if last > starts}
						<b class="font-bold text-ink">{plural(daysBetween(starts, last) + 1, 'day')}</b>, {shortDate(starts)} to {shortDate(last)}.
					{/if}
					{pauses ? 'Planned sessions on these days are paused.' : 'Planned sessions stay as they are.'}
				</p>
			</div>
		</div>
		<FormError message={error} />
		<div class="grid grid-cols-2 items-center gap-2">
			<Button variant="secondary" onclick={() => (open = false)}>Cancel</Button>
			<Button type="submit" disabled={saving || !name.trim() || !starts}>{saving ? 'Saving…' : 'Save'}</Button>
		</div>
		{#if editing}
			<Button variant="danger" disabled={saving} onclick={remove}>Delete</Button>
		{/if}
	</form>
</Sheet>
