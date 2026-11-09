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
		also = [],
		label,
		zero = false,
		max,
		steps = false,
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
		// also is a second line on the same scale, drawn lighter, such as flashes under sends.
		also?: Mark[];
		label: (v: number) => string;
		zero?: boolean;
		// max fixes the top of the scale, such as 100 for a rate.
		max?: number;
		// steps labels each whole step, as a grade, so a drop of one reads as one.
		steps?: boolean;
		height?: number;
		ticks?: string[];
		small?: boolean;
		now?: number;
		pick?: number | null;
		onpick?: (i: number) => void;
		names?: string[];
	} = $props();

	const all = $derived([...marks, ...also].map((m) => m.v));
	const lo = $derived(Math.min(...all, ...(zero || !all.length ? [0] : [])));
	const top = $derived(max ?? Math.max(...all, ...(zero ? [0] : []), lo));
	// Equal values still need a span to draw on, but only real values are labelled.
	const hi = $derived(top > lo ? top : lo + 1);
	const pad = $derived(small ? 4 : 8);
	const at = (v: number) => (pad + ((v - lo) / (hi - lo)) * (height - 2 * pad)) / height;
	const y = (v: number) => (1 - at(v)) * height;
	const x = (i: number) => ((i + 0.5) / n) * 100;
	const every = $derived(Math.ceil((top - lo) / 5) || 1);
	const grid = $derived(
		small
			? []
			: [...new Set(steps ? Array.from({ length: Math.floor((top - lo) / every) + 1 }, (_, k) => lo + k * every) : [lo, ...(zero && lo < 0 ? [0] : []), top])].map((v) => ({
					at: at(v),
					label: label(v)
				}))
	);
	const line = $derived(marks.filter((m) => m.best).sort((a, b) => a.i - b.i));
	const second = $derived([...also].sort((a, b) => a.i - b.i));
</script>

<Chart {n} {height} {grid} {ticks} {small} {now} {pick} {onpick} {names}>
	<svg class="absolute inset-0 size-full overflow-visible" viewBox="0 0 100 {height}" preserveAspectRatio="none" aria-hidden="true">
		{#if second.length}
			<polyline
				points={second.map((m) => `${x(m.i)},${y(m.v)}`).join(' ')}
				class="fill-none stroke-chart/50"
				stroke-width={small ? 1.5 : 2.5}
				stroke-linejoin="round"
				stroke-linecap="round"
				vector-effect="non-scaling-stroke"
			/>
		{/if}
		<polyline
			points={line.map((m) => `${x(m.i)},${y(m.v)}`).join(' ')}
			class="fill-none stroke-chart"
			stroke-width={small ? 1.5 : 2.5}
			stroke-linejoin="round"
			stroke-linecap="round"
			vector-effect="non-scaling-stroke"
		/>
	</svg>
	<!-- A second mark on a first one rings it, so a tie still shows both. -->
	{#each second as m, k (k)}
		<span
			class="absolute -translate-1/2 rounded-full {marks.some((o) => o.best && o.i === m.i && o.v === m.v)
				? 'z-20 size-[22px] border-2 border-chart/50'
				: 'size-2.5 bg-chart/50 ring-2 ring-surface'}"
			style:left="{x(m.i)}%"
			style:top="{y(m.v)}px"
		></span>
	{/each}
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
