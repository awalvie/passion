<script lang="ts">
	import Icon from './Icon.svelte';

	// The note spans the nearest relative box, under the button.
	let { text }: { text: string } = $props();

	let open = $state(false);
	let box: HTMLElement;
</script>

<svelte:window
	onpointerdown={(e) => open && !box.contains(e.target as Node) && (open = false)}
	onkeydown={(e) => e.key === 'Escape' && (open = false)}
/>

<span class="contents" bind:this={box}>
	<button
		type="button"
		class="-m-3 flex size-11 shrink-0 items-center justify-center rounded-full {open ? 'text-ink' : 'text-ink-3'}"
		aria-label="How this is measured"
		aria-expanded={open}
		onclick={() => (open = !open)}
	>
		<Icon name="info" size="18px" stroke={2.2} />
	</button>
	{#if open}
		<span role="note" class="absolute inset-x-0 top-full z-30 mt-2 rounded-2xl bg-ink px-3.5 py-3 text-[15px] leading-snug font-semibold text-ground shadow-card">{text}</span>
	{/if}
</span>
