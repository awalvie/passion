<script lang="ts">
	import Icon from './Icon.svelte';
	import type { Snippet } from 'svelte';

	let {
		title = '',
		heading = true,
		back,
		actions
	}: { title?: string; heading?: boolean; back?: { href: string; label: string }; actions?: Snippet } = $props();

	let left = $state(0);
	let right = $state(0);
	// Room either side of the centered title for the wider pill group, the
	// padding and a gap.
	const side = $derived(Math.max(left, right) + 24);
</script>

<header class="sticky top-0 z-30 bg-ground/85 pt-[env(safe-area-inset-top)] backdrop-blur-md">
	<div class="relative flex h-14 items-center justify-between gap-2 px-4">
		<div class="min-w-0" bind:clientWidth={left}>
			{#if back}
				<a
					href={back.href}
					data-back
					class="inline-flex h-11 max-w-full items-center gap-0.5 rounded-full bg-surface pr-4 pl-2.5 text-[15px] font-bold text-ink shadow-card-sm"
				>
					<Icon name="chevron-left" size="1.25rem" stroke={2.4} />
					<span class="truncate">{back.label}</span>
				</a>
			{/if}
		</div>
		<!-- Centered on the screen, not between the pills. -->
		{#if title}
			<svelte:element
				this={heading ? 'h1' : 'p'}
				class="pointer-events-none absolute left-1/2 -translate-x-1/2 truncate text-[15px] font-bold text-ink"
				style:max-width="calc(100% - {2 * side}px)">{title}</svelte:element
			>
		{/if}
		<div class="flex min-w-0 items-center justify-end gap-2" bind:clientWidth={right}>
			{@render actions?.()}
		</div>
	</div>
</header>
