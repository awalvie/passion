<script lang="ts">
	import type { Trend } from './progress';

	// A figure over the last 4 weeks, as a week on average, with its change on
	// the 4 weeks before. Its bars are the 8 weeks: the 4 before pale, then the
	// last 4, so each week reads against the ones before. With href it links;
	// replace keeps the link from adding to the history.
	let { label, t, unit, href, replace = false }: { label: string; t: Trend; unit: string; href?: string; replace?: boolean } = $props();

	const round = (h: number) => Math.round(h * 10) / 10;
	const change = $derived.by(() => {
		if (t.now === null || t.then === null) return null;
		const d = round(t.now - t.then);
		return d ? `${Math.abs(d)}${unit}` : 'Same';
	});
	const shown = $derived(t.weeks.filter((v) => v !== null));
	const hi = $derived(Math.max(...shown, 0.001));
</script>

<li class="min-w-0">
	<svelte:element
		this={href ? 'a' : 'div'}
		{href}
		data-sveltekit-replacestate={href && replace ? '' : undefined}
		class="flex h-full flex-col gap-0.5 rounded-3xl bg-surface p-[18px] shadow-card"
	>
		<span class="truncate text-xs font-semibold text-ink-2">{label}</span>
		<span class="flex items-baseline justify-between gap-1.5">
			<span class="digits text-xl">{t.now === null ? '–' : `${t.now}${unit}`}</span>
			{#if change}
				{@const up = t.now! > t.then!}
				<span class="flex items-center gap-1 text-xs font-semibold whitespace-nowrap text-ink-2" aria-label="{change === 'Same' ? 'Same' : `${up ? 'Up' : 'Down'} ${change}`} on the 4 weeks before">
					{#if change !== 'Same'}<svg class="size-2.5 fill-current {up ? '' : 'rotate-180'}" viewBox="0 0 10 10" aria-hidden="true"><path d="M5 1.5 9 8.5H1z" /></svg>{/if}{change}
				</span>
			{/if}
		</span>
		{#if shown.length}
			<svg class="mt-2.5 h-9 w-full lg:mt-3 lg:h-14" viewBox="0 0 84 56" preserveAspectRatio="none" aria-hidden="true">
				{#each t.weeks as v, i (i)}
					{#if v !== null}<rect x={i * 10 + (i < 4 ? 0 : 4) + 1.5} y={56 - Math.max(2, (v / hi) * 56)} width="7" height={Math.max(2, (v / hi) * 56)} rx="1.5" class={i < 4 ? 'fill-chart/30' : 'fill-chart'} />{/if}
				{/each}
			</svg>
		{/if}
	</svelte:element>
</li>
