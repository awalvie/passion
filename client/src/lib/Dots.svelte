<script lang="ts">
	import Chart from './Chart.svelte';

	// Marks in n slots, the newest on the right, with a line through each
	// slot's best. A hollow mark is a try that did not count, such as a climb
	// not sent. With zero, the scale takes in zero, so a load or a count
	// climbs true to size and an assisted (negative) load sits below the line.
	type Mark = { i: number; v: number; best?: boolean; hollow?: boolean };
	let {
		n,
		marks,
		label,
		zero = false,
		height = 120,
		ticks,
		small = false,
		now,
		pick = null,
		onpick,
		names
	}: {
		n: number;
		marks: Mark[];
		label: (v: number) => string;
		zero?: boolean;
		height?: number;
		ticks?: string[];
		small?: boolean;
		now?: number;
		pick?: number | null;
		onpick?: (i: number) => void;
		names?: string[];
	} = $props();

	const all = $derived(marks.map((m) => m.v));
	const lo = $derived(Math.min(...all, ...(zero || !all.length ? [0] : [])));
	const top = $derived(Math.max(...all, ...(zero ? [0] : []), lo));
	// Equal values still need a span to draw on, but only real values are labelled.
	const hi = $derived(top > lo ? top : lo + 1);
	const pad = $derived(small ? 4 : 8);
	const at = (v: number) => (pad + ((v - lo) / (hi - lo)) * (height - 2 * pad)) / height;
	const y = (v: number) => (1 - at(v)) * height;
	const x = (i: number) => ((i + 0.5) / n) * 100;
	const grid = $derived(
		small ? [{ at: 0, label: '' }] : [...new Set([lo, ...(zero && lo < 0 ? [0] : []), top])].map((v) => ({ at: at(v), label: label(v) }))
	);
	const line = $derived(marks.filter((m) => m.best).sort((a, b) => a.i - b.i));
</script>

<Chart {n} {height} {grid} {ticks} {small} {now} {pick} {onpick} {names}>
	<svg class="absolute inset-0 size-full overflow-visible" viewBox="0 0 100 {height}" preserveAspectRatio="none" aria-hidden="true">
		<polyline
			points={line.map((m) => `${x(m.i)},${y(m.v)}`).join(' ')}
			class="fill-none stroke-chart"
			stroke-width={small ? 1.5 : 2.5}
			stroke-linejoin="round"
			stroke-linecap="round"
			vector-effect="non-scaling-stroke"
		/>
	</svg>
	{#each marks as m, k (k)}
		<span
			class="absolute -translate-1/2 rounded-full {small
				? 'size-1.5'
				: m.best
					? 'z-10 size-3 ring-2 ring-surface'
					: 'size-2.5'} {m.best ? 'bg-chart' : m.hollow ? 'border-2 border-chart/40 bg-surface' : 'bg-chart/30'}"
			style:left="{x(m.i)}%"
			style:top="{y(m.v)}px"
		></span>
	{/each}
</Chart>
