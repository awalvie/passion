<script lang="ts">
	import Icon from './Icon.svelte';
	import type { Trend } from './progress';

	// A figure over the last 4 weeks, as a week on average, with its change on
	// the 4 weeks before. Its bars are the last 4 weeks, and a dashed line is
	// the 4 weeks before, so each week reads against them. A week with nothing
	// logged is a tick on the baseline. With href it links; replace keeps the
	// link from adding to the history.
	let { label, t, unit, href, replace = false }: { label: string; t: Trend; unit: string; href?: string; replace?: boolean } = $props();

	const round = (h: number) => Math.round(h * 10) / 10;
	// A rate moves in points, not percent.
	const step = $derived(unit === '%' ? ' pts' : unit);
	const change = $derived.by(() => {
		if (t.now === null || t.then === null) return null;
		const d = round(t.now - t.then);
		return d ? `${Math.abs(d)}${step}` : 'Same';
	});
	// The last 4 weeks are bars; the 4 weeks before are a dashed line at their figure, as the change compares them.
	const recent = $derived(t.weeks.slice(4));
	const hi = $derived(Math.max(...recent.filter((v) => v !== null), t.then ?? 0, 0.001));
	// The tallest mark stops short of the top, so the line never sits on the edge.
	const y = (v: number) => 56 - (v / hi) * 50;
</script>

<li class="min-w-0">
	<svelte:element
		this={href ? 'a' : 'div'}
		{href}
		data-sveltekit-replacestate={href && replace ? '' : undefined}
		class="flex h-full flex-col gap-0.5 rounded-3xl bg-surface p-[18px] shadow-card"
	>
		<span class="flex items-center justify-between gap-1">
			<span class="truncate text-xs font-semibold text-ink-2">{label}</span>
			{#if href}<span class="text-ink-3 lg:hidden"><Icon name="chevron-right" size="14px" stroke={2.6} /></span>{/if}
		</span>
		<span class="flex items-baseline justify-between gap-1.5">
			<span class="digits text-[26px] leading-tight">{t.now === null ? '–' : `${t.now}${unit}`}</span>
			{#if change}
				{@const up = t.now! > t.then!}
				<span class="flex items-center gap-1 text-xs font-semibold whitespace-nowrap text-ink-2" aria-label="{change === 'Same' ? 'Same' : `${up ? 'Up' : 'Down'} ${change}`} on the 4 weeks before">
					{#if change !== 'Same'}<span class={up ? '' : 'rotate-180'}><Icon name="triangle" size="10px" /></span>{/if}{change}
				</span>
			{/if}
		</span>
		<svg class="mt-2.5 h-9 w-full lg:mt-3 lg:h-14" viewBox="0 0 84 57" preserveAspectRatio="none" aria-hidden="true">
			{#each recent as v, i (i)}
				{@const x = i * 22 + 2}
				{#if v}
					<rect {x} y={56 - Math.max(2, (v / hi) * 50)} width="14" height={Math.max(2, (v / hi) * 50)} rx="2" class="fill-chart" />
				{:else if v === 0}
					<rect {x} y="55" width="14" height="1.5" rx="0.75" class="fill-chart" />
				{/if}
			{/each}
			{#if t.then !== null}<line x1="0" x2="84" y1={y(t.then)} y2={y(t.then)} class="stroke-ink-2" stroke-width="1.5" stroke-dasharray="4 3" vector-effect="non-scaling-stroke" />{/if}
		</svg>
		{#if t.then !== null}
			<span class="mt-1.5 flex items-center gap-1.5 text-xs font-semibold text-ink-2"><span class="w-3.5 border-t-2 border-dashed border-ink-2"></span>4 weeks before</span>
		{/if}
	</svelte:element>
</li>
