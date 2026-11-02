<script lang="ts">
	import { invalidateAll } from '$app/navigation';
	import { untrack } from 'svelte';
	import { describe, request } from './api';
	import Button from './Button.svelte';
	import DateField from './DateField.svelte';
	import { addDays, shortDate } from './dates';
	import FormError from './FormError.svelte';
	import { current, spanTitle, type GoalSpan, type Period, type PeriodGoal } from './goals';
	import { newId } from './id';
	import Segmented from './Segmented.svelte';
	import Sheet from './Sheet.svelte';
	import { plural } from './text';

	// Adds a goal, or changes the one in editing. A goal added from a span's
	// card goes in that span. goals are the person's, to count who shares a span.
	let {
		open = $bindable(false),
		editing,
		span: preset,
		goals,
		today
	}: { open?: boolean; editing: PeriodGoal | null; span: GoalSpan | null; goals: PeriodGoal[]; today: string } = $props();

	const periods = [
		{ value: 'week', label: 'Week' },
		{ value: 'month', label: 'Month' },
		{ value: 'year', label: 'Year' },
		{ value: 'custom', label: 'Custom' }
	] as const;

	let text = $state('');
	let period = $state<Period>('week');
	let name = $state('');
	let from = $state('');
	let to = $state('');
	// One id an opening, so a save sent again after a lost answer is the same goal.
	let id = '';
	let spanId = '';
	let saving = $state(false);
	let error = $state('');

	$effect(() => {
		if (open) untrack(reset);
	});

	function reset() {
		const span = editing?.span ?? preset;
		text = editing?.text ?? '';
		period = editing?.period ?? (preset ? 'custom' : 'week');
		name = span?.name ?? '';
		from = span?.starts ?? today;
		to = span?.ends ?? addDays(today, 6);
		error = '';
		id = editing?.id ?? newId();
		spanId = span?.id ?? newId();
	}

	// The days show only where they can change: on a new span, or on the
	// edited goal's.
	const days = $derived(period === 'custom' && (editing !== null || preset === null));
	const placeholder = $derived(from && to ? `${shortDate(from)} – ${shortDate(to)}` : '');
	const shared = $derived(editing?.span ? current(goals, today).filter((g) => g.span?.id === editing!.span!.id).length : 0);
	const labels: Record<string, string> = {
		text: 'The goal',
		period: 'The period',
		'span.id': 'The days',
		'span.name': 'The name',
		'span.starts': 'From',
		'span.ends': 'Until'
	};

	async function save() {
		saving = true;
		error = '';
		try {
			await request('PUT', `/api/v1/goals/${id}`, {
				text,
				period,
				span: period === 'custom' ? { id: spanId, name, starts: from, ends: to } : null
			});
			await invalidateAll();
			open = false;
		} catch (e) {
			error = describe(e, (f) => labels[f] ?? f);
		} finally {
			saving = false;
		}
	}

	async function remove() {
		if (!editing || !confirm(`Remove “${editing.text}”?`)) return;
		saving = true;
		error = '';
		try {
			await request('DELETE', `/api/v1/goals/${editing.id}`);
			await invalidateAll();
			open = false;
		} catch (e) {
			error = describe(e, (f) => labels[f] ?? f);
		} finally {
			saving = false;
		}
	}
</script>

<Sheet bind:open title={editing ? 'Goal' : 'New goal'}>
	<div class="flex flex-col gap-3.5">
		<div class="flex flex-col gap-3.5 rounded-3xl bg-surface p-[18px] shadow-card">
			<label class="flex flex-col gap-1.5 text-xs font-semibold text-ink-2">
				Goal
				<input class="input" bind:value={text} required maxlength="200" placeholder="Train fingers twice" />
			</label>
			{#if !editing && !preset}
				<div class="flex flex-col gap-1.5 text-xs font-semibold text-ink-2">
					For
					<Segmented label="For" items={periods.map((p) => ({ label: p.label, on: period === p.value, onclick: () => (period = p.value) }))} />
				</div>
			{:else if preset && !editing}
				<p class="flex min-h-11 items-center justify-between gap-3 text-[15px] font-bold">
					{spanTitle(preset, today).title}
					{#if preset.name}<span class="text-xs font-semibold text-ink-2">{shortDate(preset.starts)} – {shortDate(preset.ends)}</span>{/if}
				</p>
			{:else if editing && editing.period !== 'custom'}
				<p class="flex min-h-11 items-center justify-between gap-3 text-[15px] font-bold">
					Every {editing.period}<span class="text-xs font-semibold text-ink-2">Since {shortDate(editing.starts)}</span>
				</p>
			{/if}
			{#if days}
				<label class="flex flex-col gap-1.5 text-xs font-semibold text-ink-2">
					Name
					<input class="input" bind:value={name} maxlength="200" {placeholder} />
				</label>
				<div class="grid grid-cols-2 gap-3">
					<label class="flex min-w-0 flex-col gap-1.5 text-xs font-semibold text-ink-2">
						From
						<DateField bind:value={from} label="From" />
					</label>
					<label class="flex min-w-0 flex-col gap-1.5 text-xs font-semibold text-ink-2">
						Until
						<DateField bind:value={to} label="Until" min={from} />
					</label>
				</div>
			{/if}
			{#if !editing && !preset && period !== 'custom'}
				<p class="text-xs leading-snug font-semibold text-ink-2">Repeats every {period}. This cannot be changed later.</p>
			{:else if editing?.span && shared > 1}
				<p class="text-xs leading-snug font-semibold text-ink-2">The name and days are shared by {plural(shared, 'goal')}.</p>
			{/if}
		</div>
		{#if editing}
			<Button variant="danger" disabled={saving} onclick={remove}>Remove goal</Button>
		{/if}
		<FormError message={error} />
	</div>
	{#snippet footer()}
		<div class="grid grid-cols-2 items-center gap-2">
			<Button variant="secondary" onclick={() => (open = false)}>Cancel</Button>
			<Button disabled={saving || !text.trim() || (period === 'custom' && (!from || !to))} onclick={save}>
				{saving ? 'Saving…' : 'Save'}
			</Button>
		</div>
	{/snippet}
</Sheet>
