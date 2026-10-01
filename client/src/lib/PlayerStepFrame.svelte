<script lang="ts">
	import type { Snippet } from 'svelte';

	let {
		label,
		chars = 0,
		less,
		more,
		children
	}: { label: string; chars?: number; less: () => void; more: () => void; children: Snippet } = $props();

	// 16px is the floor: below it iOS zooms into a focused field.
	const size = $derived(chars <= 3 ? 'text-[32px]' : chars <= 5 ? 'text-2xl' : 'text-base');
</script>

{#snippet round(name: string, path: string, onclick: () => void)}
	<button
		type="button"
		class="relative flex size-9 shrink-0 items-center justify-center rounded-full bg-well text-ink before:absolute before:-inset-1 before:content-[''] active:opacity-70 dark:bg-[#2A332C]"
		aria-label="{name} {label}"
		{onclick}
	>
		<svg viewBox="0 0 24 24" class="size-[18px]" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" aria-hidden="true"><path d={path} /></svg>
	</button>
{/snippet}

<div class="flex min-w-0 items-center justify-between rounded-2xl bg-surface p-1.5 shadow-card-sm">
	{@render round('Less', 'M6 12h12', less)}
	<!-- Phones floor a field's font at 1em, so the size sits on the parent. -->
	<div class="flex min-h-[50px] min-w-0 flex-1 flex-col items-center justify-center {size}">
		<span class="max-w-full truncate text-xs font-semibold text-ink-2">{label}</span>
		{@render children()}
	</div>
	{@render round('More', 'M12 5v14M5 12h14', more)}
</div>
