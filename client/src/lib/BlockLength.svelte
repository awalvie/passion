<script lang="ts">
	// max is the most days a block can hold: 28, or fewer for a short cycle.
	let { value = $bindable(), max }: { value: number; max: number } = $props();
</script>

<div class="grid grid-cols-4 gap-2">
	{#each [7, 10, 14] as n (n)}
		<button
			type="button"
			class="h-12 rounded-full text-[15px] font-bold disabled:opacity-40 {value === n ? 'bg-ink text-ground' : 'bg-surface text-ink shadow-card-sm'}"
			aria-pressed={value === n}
			disabled={n > max}
			onclick={() => (value = n)}
		>
			{n} days
		</button>
	{/each}
	<input
		class="input h-12 [appearance:textfield] px-2 text-center [&::-webkit-inner-spin-button]:appearance-none [&::-webkit-outer-spin-button]:appearance-none"
		type="number"
		inputmode="numeric"
		min="1"
		max={Math.min(28, max)}
		aria-label="Days in the block"
		bind:value
		required
	/>
</div>
