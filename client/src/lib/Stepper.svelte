<script lang="ts">
	import PlayerStepFrame from './PlayerStepFrame.svelte';

	let {
		value = $bindable(),
		label,
		step = 1,
		min = 0,
		placeholder = ''
	}: { value: number | null; label: string; step?: number; min?: number; placeholder?: string } =
		$props();

	function by(delta: number) {
		const next = Math.round(((value ?? 0) + delta) * 100) / 100;
		value = Math.max(min, next);
	}
</script>

<PlayerStepFrame {label} chars={String(value ?? placeholder).length} less={() => by(-step)} more={() => by(step)}>
	<input
		type="number"
		inputmode={step % 1 ? 'decimal' : 'numeric'}
		class="w-full min-w-0 [appearance:textfield] bg-transparent text-center text-[1em] leading-[1.05] font-extrabold tracking-tight text-ink outline-none placeholder:text-ink-3 [&::-webkit-inner-spin-button]:appearance-none"
		aria-label={label}
		{placeholder}
		bind:value
	/>
</PlayerStepFrame>
