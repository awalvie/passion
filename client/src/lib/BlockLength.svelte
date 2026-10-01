<script lang="ts">
	// max is the most days a block can hold: 28, or fewer for a short cycle.
	let { value = $bindable(), max }: { value: number; max: number } = $props();

	const presets = [7, 8, 10, 14];
	const limit = $derived(Math.min(28, max));
	const custom = $derived(!presets.includes(value));

	let editing = $state(false);
	let typed = $state<number | null>(null);

	function edit() {
		typed = custom ? value : null;
		editing = true;
	}

	// An empty or out-of-range number keeps the length as it was.
	function done() {
		if (typed != null && Number.isInteger(typed) && typed >= 1 && typed <= limit) value = typed;
		editing = false;
	}

	function focus(el: HTMLInputElement) {
		el.focus();
	}
</script>

<div class="grid grid-cols-5 gap-2">
	{#each presets as n (n)}
		<button
			type="button"
			class="h-12 rounded-full text-[15px] font-bold disabled:opacity-40 {value === n ? 'bg-ink text-ground' : 'bg-surface text-ink shadow-card-sm'}"
			aria-pressed={value === n}
			disabled={n > max}
			onclick={() => (value = n)}
		>
			{n}
		</button>
	{/each}
	{#if editing}
		<input
			class="input h-12 [appearance:textfield] px-1 text-center [&::-webkit-inner-spin-button]:appearance-none [&::-webkit-outer-spin-button]:appearance-none"
			type="number"
			inputmode="numeric"
			min="1"
			max={limit}
			aria-label="Days in the block, 1 to {limit}"
			bind:value={typed}
			onblur={done}
			onkeydown={(e) => {
				if (e.key === 'Enter') {
					e.preventDefault();
					done();
				}
			}}
			use:focus
		/>
	{:else}
		<button
			type="button"
			class="h-12 rounded-full text-[15px] font-bold {custom ? 'bg-ink text-ground' : 'bg-surface text-ink shadow-card-sm'}"
			aria-pressed={custom}
			aria-label={custom ? `Custom, ${value} days` : 'Custom length'}
			onclick={edit}
		>
			{custom ? value : 'Custom'}
		</button>
	{/if}
</div>
