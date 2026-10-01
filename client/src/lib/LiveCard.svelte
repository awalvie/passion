<script lang="ts">
	import { goto } from '$app/navigation';
	import Button from './Button.svelte';
	import Menu from './Menu.svelte';
	import { currentStep, isFinished, secondsSince, setsOf, type Run } from './run';
	import { formatClock, readTimers } from './timerStore';
	import Topo from './Topo.svelte';
	import { heroTopo } from './topo';

	let { run, ondiscard }: { run: Run; ondiscard: () => void } = $props();

	let now = $state(Date.now());
	$effect(() => {
		const tick = setInterval(() => (now = Date.now()), 250);
		return () => clearInterval(tick);
	});
	const minutes = $derived(Math.floor(secondsSince(run.started_at, now) / 60));

	const current = $derived(currentStep(run));
	const at = $derived(
		current ? run.sections.findIndex((s) => s.items.some((i) => i.step?.id === current.id)) : run.sections.length
	);

	// Five sections at most, kept around the current one.
	const first = $derived(Math.max(0, Math.min(at - 2, run.sections.length - 5)));
	const shown = $derived(run.sections.slice(first, first + 5));
	const nowAt = $derived(at - first);
	const done = $derived(
		shown.map((s) => s.items.length > 0 && s.items.every((it) => it.step && isFinished(it.step)))
	);

	const heights = [36, 18, 30, 12, 26];
	let width = $state(318);
	const pad = $derived((width * 30) / 318);
	const gap = $derived(shown.length > 1 ? (width - 2 * pad) / (shown.length - 1) : width);
	const points = $derived(
		shown.map((_, i): [number, number] => [shown.length > 1 ? pad + i * gap : width / 2, heights[i]])
	);

	// A Catmull-Rom curve through the points from one index to another.
	function curve(from: number, to: number) {
		const p = points;
		let d = `M${p[from][0]} ${p[from][1]}`;
		for (let i = from; i < to; i++) {
			const [a, b, c, e] = [p[i - 1] ?? p[i], p[i], p[i + 1], p[i + 2] ?? p[i + 1]];
			d += ` C${b[0] + (c[0] - a[0]) / 6} ${b[1] + (c[1] - a[1]) / 6} ${c[0] - (e[0] - b[0]) / 6} ${c[1] - (e[1] - b[1]) / 6} ${c[0]} ${c[1]}`;
		}
		return d;
	}

	// The rest the step page started on this phone. Today only reads it.
	let endsAt = $state<number | null>(null);
	$effect.pre(() => {
		endsAt = readTimers(run.id).rest?.endsAt ?? null;
	});
	const left = $derived(endsAt === null ? 0 : (endsAt - now) / 1000);
	const total = $derived(Math.max(current?.set_rest_seconds ?? 0, left));
	const next = $derived.by(() => {
		if (!current?.sets) return '';
		const sets = Math.floor(setsOf(run, current.id).length / (current.per_side ? 2 : 1));
		return `Set ${sets + 1} of ${current.sets} next`;
	});
</script>

<article
	class="relative mx-4 mt-3.5 overflow-hidden rounded-[32px] bg-[radial-gradient(90%_70%_at_88%_0%,var(--hero-2),var(--hero)_70%)] text-on-hero shadow-[0_18px_36px_-12px_rgba(10,30,18,0.55)]"
>
	<Topo shape={heroTopo} class="absolute inset-0 h-full w-full text-[var(--hero-topo)]" />
	<div class="relative flex flex-col p-5">
		<div class="-mt-1.5 -mr-1.5 flex items-center justify-between">
			<p class="flex items-center gap-2.5 text-xs font-semibold tracking-[0.06em] uppercase">
				<i class="size-2.5 rounded-full bg-live shadow-[0_0_0_5px_var(--live-halo)]"></i>Live · {minutes} min
			</p>
			<Menu
				size="size-9"
				look="bg-white/10 text-on-hero"
				items={[
					{ label: 'Finish session', onclick: () => goto(`/run/${run.id}/finish`) },
					{ label: 'Discard session', danger: true, onclick: ondiscard }
				]}
			/>
		</div>
		<h2 class="mt-1 text-[32px] leading-tight font-extrabold tracking-tight">{run.name}</h2>
		{#if shown.length}
			<div class="relative mt-3 h-16" bind:clientWidth={width}>
				<svg class="absolute inset-0 h-16 w-full overflow-visible" viewBox="0 0 {width} 64" aria-hidden="true">
					{#if Math.min(nowAt, points.length - 1) > 0}
						<path d={curve(0, Math.min(nowAt, points.length - 1))} fill="none" stroke="var(--on-hero)" stroke-width="2.5" stroke-linecap="round" />
					{/if}
					{#if nowAt < points.length - 1}
						<path
							d={curve(nowAt, points.length - 1)}
							fill="none"
							stroke="var(--on-hero)"
							stroke-opacity="0.4"
							stroke-width="2"
							stroke-dasharray="1 6"
							stroke-linecap="round"
						/>
					{/if}
					{#each points as [x, y], i (i)}
						{#if i !== nowAt && done[i]}
							<circle cx={x} cy={y} r="6" fill="var(--on-hero)" />
						{:else if i === nowAt}
							<circle cx={x} cy={y} r="15" fill="var(--live-halo)" />
							<circle cx={x} cy={y} r="9" fill="var(--live)" />
						{:else}
							<circle cx={x} cy={y} r="6" fill="var(--hero)" stroke="var(--on-hero)" stroke-opacity="0.55" stroke-width="2" />
						{/if}
					{/each}
				</svg>
				<ol>
					{#each shown as s, i (i)}
						<li
							class="absolute bottom-0 truncate text-center text-xs {i === nowAt ? 'font-bold text-live' : 'font-semibold text-on-hero/70'}"
							style="left: {points[i][0] - gap / 2}px; width: {gap}px"
						>
							{s.name}
						</li>
					{/each}
				</ol>
			</div>
		{/if}
		{#if left > 0}
			<div class="mt-3.5 flex items-end justify-between">
				<div>
					<p class="flex items-center gap-2 text-xs font-bold tracking-[0.06em] text-rest-on-hero uppercase">
						<i class="size-2.5 rounded-full border-[2.5px] border-current"></i>Rest
					</p>
					{#if next}
						<p class="mt-0.5 text-xs font-semibold tracking-[0.06em] text-on-hero-2 uppercase">{next}</p>
					{/if}
				</div>
				<p class="text-[32px] leading-none font-extrabold tracking-tight">{formatClock(left)}</p>
			</div>
			<div class="mt-3 h-1.5 overflow-hidden rounded-full bg-white/12">
				<i class="block h-full rounded-full bg-rest-on-hero" style="width: {(1 - left / total) * 100}%"></i>
			</div>
		{/if}
		<div class={left > 0 ? 'mt-[18px]' : 'mt-4'}>
			<Button variant="live" href="/run/{run.id}">Back to session</Button>
		</div>
	</div>
</article>
