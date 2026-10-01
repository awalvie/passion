<script lang="ts">
	import { yearSpan } from './dates';
	import type { Cycle } from './plan';

	let { cycles, today }: { cycles: Cycle[]; today: string } = $props();

	const year = $derived(Number(today.slice(0, 4)));
	const months = ['J', 'F', 'M', 'A', 'M', 'J', 'J', 'A', 'S', 'O', 'N', 'D'];

	// Overlapping cycles stack in lanes, earliest start on top.
	const bars = $derived.by(() => {
		const ends: string[] = [];
		const out = [];
		for (const c of [...cycles].sort((a, b) => a.starts.localeCompare(b.starts))) {
			const span = yearSpan(c.starts, c.ends, year);
			if (!span) continue;
			let lane = ends.findIndex((e) => e < c.starts);
			if (lane < 0) lane = ends.push(c.ends) - 1;
			else ends[lane] = c.ends;
			const state: keyof typeof looks = c.ends < today ? 'past' : c.starts > today ? 'next' : 'now';
			out.push({ cycle: c, lane, state, ...span });
		}
		return out;
	});
	const lanes = $derived(Math.max(1, ...bars.map((b) => b.lane + 1)));
	// Two running cycles would print their names on top of each other, so only
	// the one that started last is named; the list below names them all.
	const named = $derived(bars.filter((b) => b.state === 'now').at(-1)?.cycle.id);
	const now = $derived(yearSpan(today, today, year)!.left);
	const looks = {
		now: 'bg-live',
		next: 'shadow-[inset_0_0_0_1.5px_var(--ink-3)]',
		past: 'bg-ink-3 opacity-50'
	};
</script>

<section class="rounded-3xl bg-surface px-[18px] pt-4 pb-3 shadow-card" aria-label="{year} cycles">
	<div class="flex items-baseline justify-between">
		<h2 class="text-[15px] font-bold">{year}</h2>
		<span class="text-xs font-semibold text-ink-2">{bars.length === 1 ? '1 cycle' : `${bars.length} cycles`}</span>
	</div>
	<div class="relative mt-3" style="height: {lanes * 14 + 18}px">
		{#each bars as b (b.cycle.id)}
			{#if b.cycle.id === named}
				<span
					class="absolute truncate text-[11px] font-bold text-ink"
					style="top: 0; left: {b.left * 100}%; max-width: {(1 - b.left) * 100}%"
				>
					{b.cycle.name}
				</span>
			{/if}
			<a
				href="/plan/cycles/{b.cycle.id}"
				class="absolute h-2.5 rounded-full {looks[b.state]}"
				style="top: {b.lane * 14 + 18}px; left: {b.left * 100}%; width: max(6px, {b.width * 100}%)"
				aria-label={b.cycle.name}
			></a>
		{/each}
		<span class="absolute top-3 bottom-[-4px] w-0.5 rounded-full bg-ink" style="left: {now * 100}%" aria-hidden="true"></span>
	</div>
	<ol class="mt-2.5 grid grid-cols-12 text-center text-[11px] font-semibold text-ink-2" aria-hidden="true">
		{#each months as m, i (i)}
			<li>{m}</li>
		{/each}
	</ol>
</section>
