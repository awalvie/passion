<script lang="ts">
	import type { Snippet } from 'svelte';

	let { title, text, onclear, children }: { title: string; text?: string; onclear?: () => void; children?: Snippet } = $props();
</script>

<section class="flex flex-col gap-3 rounded-3xl bg-surface p-[18px] shadow-card">
	<p class="text-xl font-bold tracking-tight">{title}</p>
	{#if text}<p class="text-[15px] font-semibold text-ink-2">{text}</p>{/if}
	{#if onclear}
		<button type="button" class="h-11 self-start rounded-full bg-well px-5 text-[15px] font-bold text-ink" onclick={onclear}>Clear</button>
	{/if}
	{@render children?.()}
</section>
