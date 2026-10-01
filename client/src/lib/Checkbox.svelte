<script lang="ts">
	import { tickBox } from './haptics';
	import Icon from './Icon.svelte';

	// The box alone; the label around it gives the 44 px target.
	let {
		checked = $bindable(false),
		label,
		onchange
	}: { checked?: boolean; label?: string; onchange?: (e: Event & { currentTarget: HTMLInputElement }) => void } = $props();
</script>

<span class="relative flex size-6 shrink-0">
	<input
		type="checkbox"
		class="peer size-6 cursor-pointer appearance-none rounded-lg shadow-[inset_0_0_0_2px_var(--ink-3)] checked:bg-ink checked:shadow-none focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ink"
		bind:checked
		aria-label={label}
		{onchange}
		use:tickBox
	/>
	<span class="pointer-events-none absolute inset-0 hidden items-center justify-center text-ground peer-checked:flex">
		<Icon name="check" />
	</span>
</span>
