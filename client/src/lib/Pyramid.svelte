<script lang="ts">
	import type { Step } from './progress';

	// A grade pyramid: the hardest grade on top. Each bar is centred, with its
	// flashes in the middle and the other sends split either side, so the
	// shape stays symmetric. With all, the climbs not sent sit grey at both
	// ends. most is the count a full bar stands for.
	let { rows, most, all = false }: { rows: Step[]; most: number; all?: boolean } = $props();

	const share = (n: number) => `${(n / most) * 100}%`;
</script>

<ul>
	{#each rows.toReversed() as r (r.grade)}
		{@const open = all ? (r.tried - r.sent) / 2 : 0}
		{@const side = (r.sent - r.flash) / 2}
		<!-- A grade with no sends is one grey bar, not two halves with a gap. -->
		{@const parts = r.sent
			? ([
					[open, 'bg-ink-3/40'],
					[side, 'bg-chart/50'],
					[r.flash, 'bg-chart'],
					[side, 'bg-chart/50'],
					[open, 'bg-ink-3/40']
				] as const)
			: ([[open * 2, 'bg-ink-3/40']] as const)}
		<li class="flex h-10 items-center gap-3 list-row">
			<span class="w-9 shrink-0 digits text-[15px]">{r.grade}</span>
			<span class="flex min-w-0 flex-1 justify-center gap-0.5" aria-hidden="true">
				{#each parts.filter(([n]) => n) as [n, cls], i (i)}
					<span class="h-[18px] rounded-[2px] {cls}" style:width={share(n)}></span>
				{/each}
			</span>
			<span class="shrink-0 text-right digits text-[15px] {all ? 'w-12' : 'w-9'}">
				{#if all && r.tried}
					{r.sent}<span class="text-xs font-semibold text-ink-2">/{r.tried}</span>
				{:else}
					{r.sent || ''}
				{/if}
				<span class="sr-only">{all ? ` sent of ${r.tried} tried, ${r.flash} flashed` : r.sent ? ` sent, ${r.flash} flashed` : 'none sent'}</span>
			</span>
		</li>
	{/each}
</ul>
