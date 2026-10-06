<script lang="ts">
	import { goto } from '$app/navigation';
	import Button from './Button.svelte';
	import { formatDuration } from './exercise';
	import Menu from './Menu.svelte';
	import { currentStop, secondsSince, sectionMarks, sectionShare, setsOf, type Run } from './run';
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

	const stop = $derived(currentStop(run));
	const current = $derived(stop?.step);
	const at = $derived(stop ? run.sections.findIndex((s) => s.items.includes(stop)) : run.sections.length);

	const marks = $derived(sectionMarks(run, at));

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
				<i class="size-2.5 rounded-full bg-live shadow-[0_0_0_5px_var(--live-halo)]"></i>Live · {formatDuration(Math.max(1, minutes) * 60)}
			</p>
			<Menu
				look="bg-white/10 text-on-hero"
				items={[
					{ label: 'Finish session', onclick: () => goto(`/run/${run.id}/finish`) },
					{ label: 'Discard session', danger: true, onclick: ondiscard }
				]}
			/>
		</div>
		<h2 class="mt-1 truncate text-[32px] leading-tight font-extrabold tracking-tight">{run.name}</h2>
		{#if run.sections.length}
			<div class="mt-3.5 flex gap-1.5" aria-hidden="true">
				{#each run.sections as s, i (i)}
					<span class="h-2 flex-1 overflow-hidden rounded-full {marks[i] === 'done' ? 'bg-on-hero' : 'bg-on-hero/20'}">
						{#if marks[i] === 'now'}
							<span class="block h-full rounded-full bg-live" style="width: {sectionShare(s) * 100}%"></span>
						{/if}
					</span>
				{/each}
			</div>
			<p class="mt-2.5 flex items-baseline justify-between gap-3">
				{#if stop}
					<span class="min-w-0 truncate text-lg font-extrabold text-live">{run.sections[at].name}</span>
					<span class="shrink-0 text-sm font-semibold text-on-hero-2">Section {at + 1} of {run.sections.length}</span>
				{:else}
					<span class="text-lg font-extrabold">All done</span>
				{/if}
			</p>
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
