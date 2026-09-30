<script lang="ts">
	import type { Snippet } from 'svelte';

	let {
		title = '',
		back,
		actions
	}: { title?: string; back?: { href: string; label: string }; actions?: Snippet } = $props();
</script>

<header
	class="sticky top-0 z-30 bg-ground/90 pt-[env(safe-area-inset-top)] backdrop-blur-md"
>
	<div class="grid h-11 grid-cols-[1fr_auto_1fr] items-center px-2">
		<div class="min-w-0">
			{#if back}
				<a
					href={back.href}
					class="inline-flex h-11 items-center gap-0.5 px-2 text-base text-tint"
				>
					<svg viewBox="0 0 24 24" class="size-6" fill="none" stroke="currentColor" stroke-width="2.25" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="m15 18-6-6 6-6" /></svg>
					<span class="truncate">{back.label}</span>
				</a>
			{/if}
		</div>
		<h1 class="truncate text-base font-semibold text-ink">{title}</h1>
		<div class="flex min-w-0 items-center justify-end">
			{@render actions?.()}
		</div>
	</div>
</header>
