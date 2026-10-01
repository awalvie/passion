<script lang="ts">
	import type { Snippet } from 'svelte';

	let {
		title = '',
		heading = true,
		back,
		actions
	}: { title?: string; heading?: boolean; back?: { href: string; label: string }; actions?: Snippet } = $props();
</script>

<header class="sticky top-0 z-30 bg-ground/85 pt-[env(safe-area-inset-top)] backdrop-blur-md">
	<div class="grid h-14 grid-cols-[minmax(max-content,1fr)_minmax(0,auto)_minmax(max-content,1fr)] items-center gap-2 px-4">
		<div class="min-w-0">
			{#if back}
				<a
					href={back.href}
					class="inline-flex h-11 max-w-full items-center gap-0.5 rounded-full bg-surface pr-4 pl-2.5 text-[15px] font-bold text-ink shadow-card-sm"
				>
					<svg viewBox="0 0 24 24" class="size-5 shrink-0" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="m15 18-6-6 6-6" /></svg>
					<span class="truncate">{back.label}</span>
				</a>
			{/if}
		</div>
		{#if title}
			<svelte:element this={heading ? 'h1' : 'p'} class="truncate text-[15px] font-bold text-ink">{title}</svelte:element>
		{:else}
			<span></span>
		{/if}
		<div class="flex min-w-0 items-center justify-end gap-2">
			{@render actions?.()}
		</div>
	</div>
</header>
