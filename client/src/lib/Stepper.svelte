<script lang="ts">
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

<div class="flex flex-col items-center gap-1">
	<span class="text-sm text-ink-2">{label}</span>
	<div class="flex items-center gap-1">
		<button
			type="button"
			class="flex size-11 shrink-0 items-center justify-center rounded-full bg-line text-2xl text-ink"
			aria-label="Less {label}"
			onclick={() => by(-step)}>−</button
		>
		<input
			type="number"
			inputmode="decimal"
			class="w-16 [appearance:textfield] bg-transparent text-center text-3xl font-semibold tabular-nums text-ink outline-none [&::-webkit-inner-spin-button]:appearance-none"
			aria-label={label}
			{placeholder}
			bind:value
		/>
		<button
			type="button"
			class="flex size-11 shrink-0 items-center justify-center rounded-full bg-line text-2xl text-ink"
			aria-label="More {label}"
			onclick={() => by(step)}>+</button
		>
	</div>
</div>
