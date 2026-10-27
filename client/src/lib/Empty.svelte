<script lang="ts">
	import type { Snippet } from 'svelte';

	// The card a page shows in place of an empty list: what is missing, a line
	// on what to do, and the button that does it.
	let { title, text, children }: { title: string; text?: string; children?: Snippet } = $props();
</script>

<section class="flex flex-col gap-3 rounded-3xl bg-surface p-[18px] shadow-card">
	<p class="text-xl font-bold tracking-tight">{title}</p>
	{#if text}<p class="text-[15px] font-semibold text-ink-2">{text}</p>{/if}
	{@render children?.()}
</section>
