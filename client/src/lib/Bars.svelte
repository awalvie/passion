<script lang="ts">
	import Chart from './Chart.svelte';

	// Stacked bars, one per bucket, the newest on the right. Each part names its
	// colour class. Gridlines give the scale; with values, every bar carries its
	// number instead.
	type Part = { values: number[]; cls: string };
	let {
		parts,
		height = 120,
		ticks,
		values = false,
		small = false,
		format = (n: number) => String(Math.round(n))
	}: { parts: Part[]; height?: number; ticks?: string[]; values?: boolean; small?: boolean; format?: (n: number) => string } = $props();

	const totals = $derived(parts[0].values.map((_, i) => parts.reduce((s, p) => s + p.values[i], 0)));
	const max = $derived(Math.max(...totals, 0));
	// The smallest round step that splits the tallest bar into at most four.
	const step = $derived.by(() => {
		const e = 10 ** Math.floor(Math.log10(max || 1));
		return [1, 2, 2.5, 5, 10].map((m) => m * e).find((s) => max / s <= 4)!;
	});
	const ceiling = $derived(small || values ? max || 1 : Math.ceil(max / step) * step || step);
	// Room above the tallest bar for its number.
	const plot = $derived(small ? height : height - 18);
	const grid = $derived(
		small || values
			? [{ at: 0, label: '' }]
			: Array.from({ length: Math.round(ceiling / step) + 1 }, (_, k) => ({ at: ((k * step) / ceiling) * (plot / height), label: k ? format(k * step) : '' }))
	);
	const gap = $derived(small ? 1.5 : totals.length > 16 ? 2 : 4);
</script>

<Chart n={totals.length} {height} {grid} {ticks} {small}>
	<div class="absolute inset-0 flex items-end" style:gap="{gap}px" style:padding-inline="{gap / 2}px">
		{#each totals as total, i (i)}
			{@const now = i === totals.length - 1}
			<span class="relative flex h-full min-w-0 flex-1 flex-col justify-end">
				{#if !small && total && (values || (now && totals.length <= 16))}
					<span
						class="absolute digits leading-none whitespace-nowrap inset-x-[-8px] text-center {now ? 'text-[15px] text-ink' : 'text-xs font-semibold text-ink-2'}"
						style:bottom="{(total / ceiling) * plot + 5}px">{format(total)}</span
					>
				{/if}
				{#if total}
					<span
						class="flex w-full flex-col-reverse self-center overflow-hidden {small ? 'rounded-t-[2px]' : 'max-w-7 rounded-t-[4px]'}"
						style:height="{Math.max(2, (total / ceiling) * plot)}px"
						style:gap={small ? '1px' : '2px'}
					>
						{#each parts as p, j (j)}
							{#if p.values[i]}<span class="min-h-px {p.cls}" style:flex={p.values[i]}></span>{/if}
						{/each}
					</span>
				{/if}
			</span>
		{/each}
	</div>
</Chart>
