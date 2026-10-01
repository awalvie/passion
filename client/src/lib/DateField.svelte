<script lang="ts">
	import { formatDate } from './dates';
	import Icon from './Icon.svelte';

	// The native field sits on top, see-through, so a tap opens the phone's own
	// date picker. Under it the date reads in full; a narrow native field cuts
	// off the year.
	let {
		value = $bindable(),
		label,
		name,
		min,
		disabled = false
	}: { value: string; label: string; name?: string; min?: string; disabled?: boolean } = $props();
</script>

<span class="input relative flex h-12 items-center gap-2 px-4 focus-within:outline-2 focus-within:outline-ink {disabled ? 'opacity-50' : ''}">
	<span class="shrink-0 text-ink-2"><Icon name="calendar" size="1rem" /></span>
	<span class="truncate">{value ? formatDate(value, { weekday: 'short', day: 'numeric', month: 'short' }) : 'Pick a day'}</span>
	<input
		class="absolute inset-0 size-full cursor-pointer opacity-0 disabled:cursor-default"
		type="date"
		aria-label={label}
		{name}
		bind:value
		{min}
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
