<script lang="ts">
	import { addDays, dayDate } from './dates';
	import Icon from './Icon.svelte';

	// The native field sits on top, see-through, so a tap opens the phone's own
	// date picker. Under it the date reads in full; a narrow native field cuts
	// off the year. A pill sits among buttons; given today, it names today and
	// yesterday, so a note written the next morning shows its date at a glance.
	let {
		value = $bindable(),
		label,
		name,
		min,
		max,
		disabled = false,
		pill = false,
		today
	}: { value: string; label: string; name?: string; min?: string; max?: string; disabled?: boolean; pill?: boolean; today?: string } = $props();

	const shown = $derived(
		!value ? 'Pick a day' : value === today ? 'Today' : today && value === addDays(today, -1) ? 'Yesterday' : dayDate(value)
	);
</script>

<span
	class="relative flex items-center gap-2 px-4 {pill
		? 'h-11 rounded-full bg-well text-[15px] font-bold text-ink has-[:focus-visible]:outline-2 has-[:focus-visible]:outline-ink'
		: 'input h-12 focus-within:outline-2 focus-within:outline-ink'} {disabled ? 'opacity-50' : ''}"
>
	<span class="shrink-0 text-ink-2"><Icon name="calendar" size="1rem" /></span>
	<span class="truncate">{shown}</span>
	<input
		class="absolute inset-0 size-full cursor-pointer opacity-0 disabled:cursor-default"
		type="date"
		aria-label={pill ? `${label}: ${shown}` : label}
		{name}
		bind:value
		{min}
		{max}
		{disabled}
		required
		onclick={(e) => {
			try {
				e.currentTarget.showPicker();
			} catch {
				// Some browsers open the picker on their own and refuse a second ask.
			}
		}}
	/>
</span>
