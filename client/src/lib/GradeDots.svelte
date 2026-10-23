<script lang="ts">
	import Chart from './Chart.svelte';

	// The hardest send in each bucket as a dot, the dots joined by a line.
	let {
		points,
		grades,
		height = 120,
		ticks,
		small = false
	}: { points: (number | null)[]; grades: string[]; height?: number; ticks?: string[]; small?: boolean } = $props();

	const all = $derived(points.filter((v): v is number => v !== null));
	const lo = $derived(all.length ? Math.min(...all) : 0);
	const hi = $derived(all.length ? Math.max(Math.max(...all), lo + 2) : 2);
	const pad = $derived(small ? 4 : 8);
	const at = (v: number) => (pad + ((v - lo) / (hi - lo)) * (height - 2 * pad)) / height;
	const y = (v: number) => (1 - at(v)) * height;
	const x = (i: number) => ((i + 0.5) / points.length) * 100;
	// Every grade between the lowest and the highest, or every other when they
	// would crowd.
	const grid = $derived(
		small
			? [{ at: 0, label: '' }]
			: Array.from({ length: hi - lo + 1 }, (_, k) => lo + k)
					.filter((g) => (g - lo) % (hi - lo > 6 ? 2 : 1) === 0)
					.map((g) => ({ at: at(g), label: grades[g] }))
	);
	const sends = $derived(points.flatMap((v, i) => (v === null ? [] : [{ i, v }])));
</script>

<Chart n={points.length} {height} {grid} {ticks} {small}>
	<svg class="absolute inset-0 size-full overflow-visible" viewBox="0 0 100 {height}" preserveAspectRatio="none" aria-hidden="true">
		<polyline
			points={sends.map((s) => `${x(s.i)},${y(s.v)}`).join(' ')}
			class="fill-none stroke-rest-on-hero"
			stroke-width={small ? 1.5 : 2.5}
			stroke-linejoin="round"
			stroke-linecap="round"
			vector-effect="non-scaling-stroke"
		/>
	</svg>
	{#each sends as s (s.i)}
		<span
			class="absolute -translate-1/2 rounded-full bg-rest-on-hero {small ? 'size-1.5' : 'size-3 ring-2 ring-surface'}"
			style:left="{x(s.i)}%"
			style:top="{y(s.v)}px"
		></span>
	{/each}
</Chart>
