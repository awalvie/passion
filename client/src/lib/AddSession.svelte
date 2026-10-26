<script lang="ts">
	import { invalidateAll } from '$app/navigation';
	import { describe, request, RequestFailed } from './api';
	import Button from './Button.svelte';
	import { dayDate } from './dates';
	import DayPicker from './DayPicker.svelte';
	import FormError from './FormError.svelte';
	import { notice } from './notice.svelte';
	import type { ScheduledDay } from './plan';
	import Select from './Select.svelte';
	import Sheet from './Sheet.svelte';
	import SessionIcon from './SessionIcon.svelte';
	import type { SessionTemplate } from './template';
	import { plural } from './text';

	// Puts one session on the days picked on a small calendar. days are the
	// plan's rows, so a day that already holds the session cannot be picked.
	let {
		open = $bindable(false),
		templates,
		days,
		today,
		day
	}: { open?: boolean; templates: SessionTemplate[]; days: ScheduledDay[]; today: string; day: string } = $props();

	let template = $state('');
	let picked = $state<string[]>([]);
	let adding = $state(false);
	let error = $state('');

	// Each opening starts over on the day the plan shows.
	$effect(() => {
		if (!open) return;
		template = templates[0]?.id ?? '';
		picked = [day < today ? today : day];
		error = '';
	});

	const chosen = $derived(templates.find((t) => t.id === template));
	const held = $derived(new Set(days.filter((d) => d.template === template).map((d) => d.local_date)));
	const sorted = $derived([...picked].filter((d) => !held.has(d)).sort());

	function pick(date: string) {
		picked = picked.includes(date) ? picked.filter((d) => d !== date) : [...picked, date];
	}

	// One request a day: each is safe to send again, and a day that already
	// holds the session only counts as left out.
	async function add() {
		adding = true;
		error = '';
		let added = 0;
		let skipped = 0;
		try {
			for (const date of sorted) {
				try {
					await request('POST', '/api/v1/scheduled-sessions', { template, local_date: date });
					added++;
				} catch (e) {
					if (!(e instanceof RequestFailed && e.error.fields?.local_date)) throw e;
					skipped++;
				}
			}
			open = false;
			notice.text = [added === 1 ? 'Session added' : `${plural(added, 'session')} added`, skipped ? `${skipped} already there` : '']
				.filter(Boolean)
				.join(' · ');
		} catch (e) {
			error = describe(e, (f) => (f === 'template' ? 'That session' : f === 'local_date' ? 'That day' : f));
		} finally {
			// Days added before a failure show in the plan all the same.
			if (added) await invalidateAll();
			adding = false;
		}
	}
</script>

{#snippet option(t: { value: string; label: string; icon: string | null })}
	<SessionIcon icon={t.icon} name={t.label} size={28} />
	<span class="truncate">{t.label}</span>
{/snippet}

<Sheet bind:open title="Add a session" wide>
	<form
		class="flex flex-col gap-3.5"
		onsubmit={(e) => {
			e.preventDefault();
			add();
		}}
	>
		<div class="flex flex-col gap-3.5 lg:grid lg:grid-cols-[minmax(0,1fr)_324px] lg:items-start">
			<div class="flex flex-col gap-4 rounded-3xl bg-surface p-[18px] shadow-card">
				<label class="flex flex-col gap-1.5 text-xs font-semibold text-ink-2">
					Session
					<Select bind:value={template} label="Session" options={templates.map((t) => ({ value: t.id, label: t.name, icon: t.icon }))} {option} />
				</label>
			</div>
			<div>
				<div class="rounded-3xl bg-surface px-2.5 pt-1.5 pb-2.5 shadow-card">
					{#key open}
					<DayPicker
						start={day < today ? today : day}
						{today}
						color={chosen?.color ?? 'var(--ink)'}
						min={today}
						mark={(date) => (held.has(date) ? 'hold' : picked.includes(date) ? 'pick' : '')}
						onpick={pick}
					/>
					{/key}
				</div>
				<p class="mt-3 px-1 text-[15px] leading-normal font-medium text-ink-2">
					{#if sorted.length}
						<b class="font-bold text-ink">{plural(sorted.length, 'day')}</b>: {sorted.map(dayDate).join(', ')}.
					{:else}
						Pick the days on the calendar.
					{/if}
				</p>
			</div>
		</div>
		<FormError message={error} />
		<div class="grid grid-cols-2 items-center gap-2">
			<Button variant="secondary" onclick={() => (open = false)}>Cancel</Button>
			<Button type="submit" disabled={adding || !sorted.length || !template}>{adding ? 'Adding…' : 'Add'}</Button>
		</div>
	</form>
</Sheet>
