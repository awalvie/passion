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

	// A held button steps again every 100 ms after 400 ms. The click that ends
	// a hold is not one more step; a click from the keyboard still is.
	let timer: ReturnType<typeof setTimeout> | undefined;
	let held = false;
	$effect(() => () => clearTimeout(timer));

	function hold(e: PointerEvent, step: () => void) {
		if (e.button > 0) return;
		held = false;
		const next = (wait: number) => {
			timer = setTimeout(() => {
				held = true;
				step();
				next(100);
			}, wait);
		};
		next(400);
	}

	function stop() {
		clearTimeout(timer);
	}
</script>

{#snippet round(name: string, path: string, onclick: () => void)}
	<button
		type="button"
		class="relative flex size-9 shrink-0 items-center justify-center rounded-full bg-well text-ink select-none [-webkit-touch-callout:none] before:absolute before:-inset-1 before:content-[''] active:opacity-70 dark:bg-[#2A332C]"
		aria-label="{name} {label}"
		onpointerdown={(e) => hold(e, onclick)}
		onpointerup={stop}
		onpointerleave={stop}
		onpointercancel={stop}
		oncontextmenu={(e) => e.preventDefault()}
		onclick={() => {
			if (held) held = false;
			else onclick();
		}}
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
