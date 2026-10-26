<script lang="ts">
	import type { Step } from './progress';

	// A grade pyramid: the hardest grade on top, one bar of sends for each
	// grade. Each bar is centred, with its flashes in the middle and the other
	// sends split either side, so the shape stays symmetric.
	let { rows }: { rows: Step[] } = $props();

	const top = $derived(Math.max(...rows.map((r) => r.sent)));
	const share = (n: number) => `${(n / top) * 100}%`;
</script>

<ul>
	{#each rows.toReversed() as r (r.grade)}
		{@const side = (r.sent - r.flash) / 2}
		<li class="flex h-10 items-center gap-3 list-row">
			<span class="w-9 shrink-0 digits text-[15px]">{r.grade}</span>
			<span class="flex min-w-0 flex-1 justify-center gap-0.5" aria-hidden="true">
				{#if r.flash && side}
					<span class="h-[18px] rounded-[2px] bg-chart/50" style:width={share(side)}></span>
					<span class="h-[18px] rounded-[2px] bg-chart" style:width={share(r.flash)}></span>
					<span class="h-[18px] rounded-[2px] bg-chart/50" style:width={share(side)}></span>
				{:else if r.sent}
					<span class="h-[18px] rounded-[2px] {r.flash ? 'bg-chart' : 'bg-chart/50'}" style:width={share(r.sent)}></span>
				{/if}
			</span>
			<span class="w-9 shrink-0 text-right digits text-[15px]">
				{r.sent || ''}<span class="sr-only">{r.sent ? ` sent, ${r.flash} flashed` : 'none sent'}</span>
			</span>
		</li>
	{/each}
</ul>
