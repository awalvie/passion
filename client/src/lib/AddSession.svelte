<script lang="ts">
	import { invalidateAll } from '$app/navigation';
	import { untrack } from 'svelte';
	import { describe, request, RequestFailed } from './api';
	import Button from './Button.svelte';
	import DateField from './DateField.svelte';
	import { addDays, dayDate, shortDate, weekdayLetters } from './dates';
	import DayPicker from './DayPicker.svelte';
	import FormError from './FormError.svelte';
	import { newId } from './id';
	import { notice } from './notice.svelte';
	import { offOn, type ScheduledDay, type TimeOff } from './plan';
	import { fallsOn, nthDay, repeatDays, ruleText, weekdayNames, weekdayOf, type RepeatRule } from './repeat';
	import Segmented from './Segmented.svelte';
	import Select from './Select.svelte';
	import Sheet from './Sheet.svelte';
	import SessionIcon from './SessionIcon.svelte';
	import Stepper from './Stepper.svelte';
	import type { SessionTemplate } from './template';
	import { plural } from './text';

	// Puts one session on the days picked on a small calendar, or on the days a
	// repeat falls on. days are the plan's rows, so a day that already holds the
	// session cannot be picked.
	let {
		open = $bindable(false),
		templates,
		days,
		timeOff,
		today,
		day
	}: { open?: boolean; templates: SessionTemplate[]; days: ScheduledDay[]; timeOff: TimeOff[]; today: string; day: string } = $props();

	let template = $state('');
	let picked = $state<string[]>([]);
	let repeats = $state(false);
	let starts = $state('');
	let every = $state<number | null>(1);
	let unit = $state<RepeatRule['unit']>('week');
	let weekdays = $state<number[]>([]);
	let ending = $state<'never' | 'on' | 'after'>('never');
	let until = $state('');
	let times = $state<number | null>(10);
	// One id an opening, so a save sent again after a lost answer is the same repeat.
	let id = '';
	let adding = $state(false);
	let error = $state('');

	// Each opening starts over on the day the plan shows. A reload while it is
	// open, after a day was added, leaves the form as it is.
	$effect(() => {
		if (open) untrack(reset);
	});

	function reset() {
		const first = day < today ? today : day;
		template = templates[0]?.id ?? '';
		picked = [first];
		repeats = false;
		starts = first;
		every = 1;
		unit = 'week';
		weekdays = [weekdayOf(first)];
		ending = 'never';
		until = addDays(first, 27);
		times = 10;
		error = '';
		id = newId();
	}

	const chosen = $derived(templates.find((t) => t.id === template));
	const held = $derived(new Set(days.filter((d) => d.template === template).map((d) => d.local_date)));
	const sorted = $derived(picked.filter((d) => !held.has(d)).sort());

	// A typed number can be anything; the rule takes a whole one from 1 to 99.
	const whole = (n: number | null) => Math.min(99, Math.max(1, Math.round(n ?? 1)));
	const rule = $derived.by((): RepeatRule => {
		const r: RepeatRule = {
			starts,
			ends: null,
			every: whole(every),
			unit,
			weekdays: unit === 'week' ? [...weekdays].sort() : []
		};
		if (ending === 'on') r.ends = until;
		if (ending === 'after') r.ends = nthDay(r, whole(times));
		return r;
	});
	const falls = $derived(rule.unit === 'day' || rule.weekdays.length > 0);
	const firstDay = $derived(nthDay(rule, 1));
	const ruleDays = $derived(rule.ends ? repeatDays(rule, starts, rule.ends).filter((d) => !held.has(d)) : []);
	const count = $derived(ruleDays.length);
	const paused = (dates: string[]) => dates.filter((d) => offOn(timeOff, d)?.pauses).length;
	const pausedNote = (n: number) => (n === 1 ? ' 1 falls in time off and is paused.' : n ? ` ${n} fall in time off and are paused.` : '');

	function pick(date: string) {
		if (repeats) starts = date;
		else picked = picked.includes(date) ? picked.filter((d) => d !== date) : [...picked, date];
	}

	// The repeat's first day is filled in, the later ones ringed.
	function mark(date: string) {
		if (!repeats) return held.has(date) ? 'hold' : picked.includes(date) ? 'pick' : '';
		if (!fallsOn(rule, date)) return '';
		return held.has(date) ? 'hold' : date === firstDay ? 'pick' : 'gen';
	}

	function toggleDay(w: number) {
		weekdays = weekdays.includes(w) ? weekdays.filter((x) => x !== w) : [...weekdays, w];
	}

	const labels: Record<string, string> = {
		template: 'That session',
		local_date: 'That day',
		starts: 'The start',
		ends: 'The end',
		every: 'Every',
		weekdays: 'The days'
	};

	// One request a day: each is safe to send again, and a day that already
	// holds the session only counts as left out.
	async function addPicked() {
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
		} finally {
			// Days added before a failure show in the plan all the same.
			if (added) await invalidateAll();
		}
	}

	async function addRepeat() {
		const out = await request<{ left_out: unknown[] }>('PUT', `/api/v1/repeats/${id}`, { template, ...rule });
		await invalidateAll();
		open = false;
		notice.text = ['Repeat added', out.left_out.length ? `${out.left_out.length} already there` : ''].filter(Boolean).join(' · ');
	}

	async function add() {
		adding = true;
		error = '';
		try {
			await (repeats ? addRepeat() : addPicked());
		} catch (e) {
			error = describe(e, (f) => labels[f] ?? f);
		} finally {
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
				<div class="flex flex-col gap-1.5 text-xs font-semibold text-ink-2">
					Repeat
					<Segmented
						label="Repeat"
						items={[
							{ label: "Doesn't repeat", on: !repeats, onclick: () => (repeats = false) },
							{ label: 'Repeats', on: repeats, onclick: () => (repeats = true) }
						]}
					/>
				</div>
				{#if repeats}
					<div class="grid grid-cols-2 items-end gap-3">
						<Stepper bind:value={every} label="Every" min={1} />
						<Select
							bind:value={unit}
							label="Unit"
							options={[
								{ value: 'day', label: every === 1 ? 'day' : 'days' },
								{ value: 'week', label: every === 1 ? 'week' : 'weeks' }
							]}
						/>
					</div>
					{#if unit === 'week'}
						<div class="flex flex-col gap-1.5 text-xs font-semibold text-ink-2">
							On
							<div class="flex justify-between" role="group" aria-label="Days of the week">
								{#each weekdayLetters as letter, i (i)}
									{@const on = weekdays.includes(i + 1)}
									<button
										type="button"
										class="flex size-11 items-center justify-center rounded-full text-[15px] font-bold {on ? 'bg-ink text-ground' : 'bg-well text-ink-2'}"
										aria-pressed={on}
										aria-label={weekdayNames[i]}
										onclick={() => toggleDay(i + 1)}
									>
										{letter}
									</button>
								{/each}
							</div>
						</div>
					{/if}
					<div class="flex flex-col gap-1.5 text-xs font-semibold text-ink-2">
						Ends
						<Segmented
							label="Ends"
							items={[
								{ label: 'Never', on: ending === 'never', onclick: () => (ending = 'never') },
								{ label: 'On a date', on: ending === 'on', onclick: () => (ending = 'on') },
								{ label: 'After', on: ending === 'after', onclick: () => (ending = 'after') }
							]}
						/>
					</div>
					{#if ending === 'on'}
						<DateField bind:value={until} label="Last day" min={starts} />
					{:else if ending === 'after'}
						<Stepper bind:value={times} label="Times" min={1} />
					{/if}
				{/if}
			</div>
			<div>
				<div class="rounded-3xl bg-surface px-2.5 pt-1.5 pb-2.5 shadow-card">
					{#key open}
						<DayPicker
							start={starts || today}
							{today}
							color={chosen?.color}
							min={today}
							{mark}
							off={(date) => !!offOn(timeOff, date)}
							onpick={pick}
						/>
					{/key}
				</div>
				<p class="mt-3 px-1 text-[15px] leading-normal font-medium text-ink-2">
					{#if !repeats}
						{#if sorted.length}
							<b class="font-bold text-ink">{plural(sorted.length, 'day')}</b>: {sorted.map(dayDate).join(', ')}.{pausedNote(paused(sorted))}
						{:else}
							Pick the days on the calendar.
						{/if}
					{:else if !falls}
						Pick at least one day of the week.
					{:else if !rule.ends}
						<b class="font-bold text-ink">No end</b>, {ruleText(rule)} from {shortDate(starts)}.
					{:else if count}
						<b class="font-bold text-ink">{count === 1 ? 'Once' : `${count} times`}</b>, {ruleText(rule)} from {shortDate(starts)} to {shortDate(rule.ends)}.{pausedNote(paused(ruleDays))}
					{:else}
						It does not fall on a free day before it ends.
					{/if}
				</p>
			</div>
		</div>
		<FormError message={error} />
		<div class="grid grid-cols-2 items-center gap-2">
			<Button variant="secondary" onclick={() => (open = false)}>Cancel</Button>
			<Button type="submit" disabled={adding || !template || (repeats ? !falls || (!!rule.ends && !count) : !sorted.length)}>
				{adding ? 'Adding…' : 'Add'}
			</Button>
		</div>
	</form>
</Sheet>
