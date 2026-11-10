<script lang="ts">
	import Chart from './Chart.svelte';

	// Marks in n slots, the newest on the right, with a line through each
	// slot's best. With now, dashes join the slot under way, as it is not over. A hollow mark is a try that did not count, such as a climb
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
		limit,
		spread = false,
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
		// also is a second line on the same scale, dashed with small marks, such as flashes under sends.
		also?: Mark[];
		label: (v: number) => string;
		zero?: boolean;
		// max fixes the top of the scale, such as 100 for a rate.
		max?: number;
		// steps labels each whole step, as a grade, so a drop of one reads as one.
		steps?: boolean;
		// limit is the last step there is, as the hardest grade of a scale. With
		// it, a line over fewer than four steps takes in the steps either side,
		// so a move of one does not fill the chart.
		limit?: number;
		// spread runs the marks edge to edge, the first on the left and the
		// last on the right, for a trend over sessions rather than time.
		spread?: boolean;
		height?: number;
		ticks?: string[];
		small?: boolean;
		now?: number;
		pick?: number | null;
		onpick?: (i: number) => void;
		names?: string[];
	} = $props();

	const all = $derived([...marks, ...also].map((m) => m.v));
	const room = $derived.by(() => {
		let a = Math.min(...all, ...(zero || !all.length ? [0] : []));
		let b = max ?? Math.max(...all, ...(zero ? [0] : []), a);
		if (steps && limit !== undefined)
			while (b - a < 3 && (a > 0 || b < limit)) {
				if (a > 0) a--;
				if (b - a < 3 && b < limit) b++;
			}
		return [a, b];
	});
	const lo = $derived(room[0]);
	const top = $derived(room[1]);
	// Equal values still need a span to draw on, but only real values are labelled.
	const hi = $derived(top > lo ? top : lo + 1);
	// A flat line with no zero to show sits in the middle, not at the floor.
	const base = $derived(top > lo || zero || max !== undefined ? lo : lo - 1);
	const pad = $derived(small ? 4 : 8);
	const at = (v: number) => (pad + ((v - base) / (hi - base)) * (height - 2 * pad)) / height;
	const y = (v: number) => (1 - at(v)) * height;
	const x = (i: number) => (spread ? (n > 1 ? (i / (n - 1)) * 100 : 50) : ((i + 0.5) / n) * 100);
	// At most five labels, and fewer on a short chart, so they never touch.
	// Steps count down from the top, so the hardest grade is always named.
	const every = $derived(Math.ceil((top - lo) / Math.max(1, Math.min(5, Math.floor((height - 2 * pad) / 16)))) || 1);
	const grid = $derived(
		small
			? []
			: [...new Set(steps ? Array.from({ length: Math.floor((top - lo) / every) + 1 }, (_, k) => top - k * every) : [lo, ...(zero && lo < 0 ? [0] : []), top])].map((v) => ({
					at: at(v),
					label: label(v)
				}))
	);
	const line = $derived(marks.filter((m) => m.best).sort((a, b) => a.i - b.i));
	// The week under way is not over, so dashes join its mark.
	const going = $derived(now !== undefined && now >= 0 && !spread ? now : -1);
	const settled = $derived(line.filter((m) => m.i !== going));
	const tail = $derived.by(() => {
		const m = line.find((m) => m.i === going);
		const prev = settled.filter((p) => p.i < going).at(-1);
		return m && prev ? [prev, m] : [];
	});
	const second = $derived([...also].sort((a, b) => a.i - b.i));
</script>

<Chart {n} {height} {grid} {ticks} {small} start={steps} {now} {pick} {onpick} {names}>
	<svg class="absolute inset-0 size-full overflow-visible" viewBox="0 0 100 {height}" preserveAspectRatio="none" aria-hidden="true">
		{#if second.length}
			<polyline
				points={second.map((m) => `${x(m.i)},${y(m.v)}`).join(' ')}
				class="fill-none stroke-chart"
				stroke-width={small ? 1.5 : 2}
				stroke-dasharray={small ? undefined : '4 4'}
				stroke-linejoin="round"
				stroke-linecap="round"
				vector-effect="non-scaling-stroke"
			/>
		{/if}
		<polyline
			points={settled.map((m) => `${x(m.i)},${y(m.v)}`).join(' ')}
			class="fill-none stroke-chart"
			stroke-width={small ? 1.5 : 2.5}
			stroke-linejoin="round"
			stroke-linecap="round"
			vector-effect="non-scaling-stroke"
		/>
		{#if tail.length}
			<polyline
				points={tail.map((m) => `${x(m.i)},${y(m.v)}`).join(' ')}
				class="fill-none stroke-chart"
				stroke-width={small ? 1.5 : 2.5}
				stroke-dasharray="2 5"
				stroke-linecap="round"
				vector-effect="non-scaling-stroke"
			/>
		{/if}
	</svg>
	<!-- A second mark on a first one rings it, so a tie still shows both. -->
	{#each second as m, k (k)}
		<span
			class="absolute -translate-1/2 rounded-full {marks.some((o) => o.best && o.i === m.i && o.v === m.v)
				? 'z-20 size-[22px] border-2 border-chart'
				: 'size-2 bg-chart'}"
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
