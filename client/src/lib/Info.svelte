<script lang="ts">
	import Icon from './Icon.svelte';

	// A note on how a figure is measured. A tap opens it; under a mouse a hover
	// does too. It sits in a relative box, and spans it under the button.
	let { text }: { text: string } = $props();

	let open = $state(false);
	let held = false;
	let box: HTMLElement;

	function outside(e: PointerEvent) {
		if (open && !box.contains(e.target as Node)) open = held = false;
	}
</script>

<svelte:window onpointerdown={outside} onkeydown={(e) => e.key === 'Escape' && (open = held = false)} />

<span class="contents" bind:this={box}>
	<button
		type="button"
		class="-m-3 flex size-11 shrink-0 items-center justify-center rounded-full {open ? 'text-ink' : 'text-ink-3'}"
		aria-label="How this is measured"
		aria-expanded={open}
		onclick={() => (open = held = !held)}
		onpointerenter={(e) => e.pointerType === 'mouse' && (open = true)}
		onpointerleave={(e) => e.pointerType === 'mouse' && !held && (open = false)}
	>
		<Icon name="info" size="18px" stroke={2.2} />
	</button>
	{#if open}
		<span role="note" class="absolute inset-x-0 top-full z-30 mt-2 rounded-2xl bg-ink px-3.5 py-3 text-[15px] leading-snug font-semibold text-ground shadow-card">{text}</span>
	{/if}
</span>
