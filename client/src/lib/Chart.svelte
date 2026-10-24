<script lang="ts">
	import type { Snippet } from 'svelte';

	// The frame every chart shares: n slots, the newest on the right, quiet
	// gridlines labelled at the left, and time ticks below. The
	// plot is drawn by children over the full width, slot i centred at
	// (i + 0.5) / n.
	let {
		n,
		height,
		grid,
		ticks,
		small = false,
		now,
		children
	}: {
		n: number;
		height: number;
		// at is how far up the plot the line sits, 0 to 1.
		grid: { at: number; label: string }[];
		ticks?: string[];
		small?: boolean;
		// now is the slot holding today, -1 when the range is over. It is the
		// newest slot unless given.
		now?: number;
		children: Snippet;
	} = $props();

	const at = $derived(now ?? n - 1);

	const top = (at: number) => `${(1 - at) * height}px`;
</script>

<div class="flex {small ? '' : 'gap-2'}">
	{#if grid.some((g) => g.label)}
		<div class="relative min-w-7 shrink-0 text-xs" style:width="{Math.max(...grid.map((g) => g.label.length))}ch" style:height="{height}px">
			{#each grid as g (g.at)}
				<span class="absolute right-0 -translate-y-1/2 digits text-xs leading-none font-semibold text-ink-3" style:top={top(g.at)}>{g.label}</span>
			{/each}
		</div>
	{/if}
	<div class="relative min-w-0 flex-1">
		<div class="relative" style:height="{height}px">
			{#each grid as g (g.at)}
				<span class="absolute inset-x-0 border-t {g.at === 0 ? 'border-ink-3/35' : 'border-line'}" style:top={top(g.at)}></span>
			{/each}
			{@render children()}
		</div>
		{#if ticks}
			<div class="relative mt-2 h-3">
				{#each ticks as t, i (i)}
					{#if t}
						<span
							class="absolute -translate-x-1/2 digits text-xs leading-none font-semibold whitespace-nowrap {i === at ? 'text-ink' : 'text-ink-3'}"
							style:left="{((i + 0.5) / n) * 100}%">{t}</span
						>
					{/if}
				{/each}
			</div>
		{/if}
	</div>
</div>
