<script lang="ts">
	import Button from './Button.svelte';
	import type { DayActions } from './dayActions.svelte';
	import { formatDate, shortDate } from './dates';
	import DateField from './DateField.svelte';
	import FormError from './FormError.svelte';
	import { ruleText, type Repeat } from './repeat';
	import Sheet from './Sheet.svelte';

	// The questions a day's actions ask: which days of a repeat go, and which
	// day a session moves to.
	let { day, repeats }: { day: DayActions; repeats: Repeat[] } = $props();

	const endingRepeat = $derived(repeats.find((r) => r.id === day.ending?.repeat));
</script>

<Sheet bind:open={day.endOpen} title="Remove {day.ending?.template_name ?? ''}?">
	{#if day.ending}
		{@const d = day.ending}
		<div class="flex flex-col gap-3.5">
			{#if endingRepeat}
				<p class="px-1 text-[15px] leading-normal font-medium text-ink-2">It repeats {ruleText(endingRepeat)}.</p>
			{/if}
			<FormError message={day.error.id === d.id ? day.error.message : ''} />
			<div class="flex flex-col gap-2">
				<Button variant="secondary" disabled={day.doing !== null} onclick={() => day.remove(d)}>Only {formatDate(d.local_date, { weekday: 'long' })}, {shortDate(d.local_date)}</Button>
				<Button variant="danger" disabled={day.doing !== null} onclick={() => day.remove(d, true)}>This and later ones</Button>
			</div>
		</div>
	{/if}
</Sheet>

<Sheet bind:open={day.moveOpen} title="Move {day.moving?.template_name ?? ''}">
	{#if day.moving}
		{@const d = day.moving}
		<form
			class="flex flex-col gap-3.5"
			onsubmit={(e) => {
				e.preventDefault();
				day.move(d, day.moveTo);
			}}
		>
			<label class="flex flex-col gap-1.5 rounded-3xl bg-surface p-[18px] text-xs font-semibold text-ink-2 shadow-card">
				New day
				<DateField bind:value={day.moveTo} label="New day" />
			</label>
			<FormError message={day.error.id === d.id ? day.error.message : ''} />
			<div class="grid grid-cols-2 items-center gap-2">
				<Button variant="secondary" onclick={() => (day.moveOpen = false)}>Cancel</Button>
				<Button type="submit" disabled={day.doing !== null}>{day.doing === `move:${d.id}` ? 'Moving…' : 'Move'}</Button>
			</div>
		</form>
	{/if}
</Sheet>
