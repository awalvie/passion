<script lang="ts">
	import { untrack } from 'svelte';
	import { request } from './api';
	import Button from './Button.svelte';
	import RestCard from './RestCard.svelte';
	import { isFinished, setsOf, type HistorySession, type RunStep, type SetFields } from './run';
	import { openRun } from './runState.svelte';
	import Stepper from './Stepper.svelte';

	let { step }: { step: RunStep } = $props();

	// A per-side set logs two rows, left then right. The server has no side
	// column, so the order carries it.
	const sides = $derived(step.per_side ? 2 : 1);
	const logged = $derived(setsOf(openRun.run!, step.id));
	const planned = $derived((step.sets ?? 0) * sides);
	const row = $derived(logged.length + 1);
	const number = $derived(Math.ceil(row / sides));
	const side = (n: number) => (n % 2 ? 'Left' : 'Right');

	let reps = $state<number | null>(null);
	let weight = $state<number | null>(null);
	let lastTime = $state<HistorySession | null>(null);

	// The newest finished run that logged this exercise. Without a signal the
	// sets start from the plan.
	$effect(() => {
		const exercise = step.exercise;
		let current = true;
		lastTime = null;
		request<{ sessions: HistorySession[] }>('GET', `/api/v1/exercises/${exercise}/history`)
			.then((h) => {
				if (!current) return;
				// A late answer fills in only what the person has not changed yet.
				const untouched = untrack(() => {
					const [r, w] = prefill();
					return reps === r && weight === w;
				});
				lastTime = h.sessions.find((s) => s.sets.length) ?? null;
				if (untouched) [reps, weight] = untrack(prefill);
			})
			.catch(() => {});
		return () => (current = false);
	});

	// Each set starts from the one before it, the first from the same set last
	// time, and else from the plan.
	function prefill(): [number | null, number | null] {
		const before = logged.at(-1);
		const then = lastTime?.sets[logged.length] ?? lastTime?.sets.at(-1);
		return [before?.reps ?? then?.reps ?? step.reps, before?.weight_kg ?? then?.weight_kg ?? null];
	}

	$effect.pre(() => {
		void step.id;
		void logged.length;
		[reps, weight] = untrack(prefill);
	});

	let rest: ReturnType<typeof RestCard>;

	function log() {
		const rows = logged.length + 1;
		openRun.setSets(step, [...logged, { reps, seconds: null, weight_kg: weight }]);
		if (planned && rows >= planned) openRun.finish(step);
		else if (rows % sides === 0) rest.start();
	}

	const done = $derived(Array.from({ length: Math.ceil(logged.length / sides) }, (_, i) => logged.slice(i * sides, (i + 1) * sides)));
	const todo = $derived(isFinished(step) ? [] : Array.from({ length: Math.max(0, (step.sets ?? 0) - number) }, (_, i) => number + 1 + i));

	function short(s: SetFields) {
		if (s.reps !== null && s.weight_kg !== null) return `${s.reps} × ${s.weight_kg} kg`;
		return describeSet(s) || 'Done';
	}

	function describeSet(s: SetFields) {
		return [s.reps !== null && `${s.reps} reps`, s.weight_kg !== null && `${s.weight_kg} kg`]
			.filter(Boolean)
			.join(' · ');
	}
</script>

{#snippet tick()}
	<span class="flex size-6 shrink-0 items-center justify-center rounded-full bg-ink text-ground dark:bg-ink-2">
		<svg viewBox="0 0 24 24" class="size-3" fill="none" stroke="currentColor" stroke-width="3.2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M5 12.5l4.5 4.5L19 7" /></svg>
	</span>
{/snippet}

{#snippet sideLabel(text: string)}
	<span class="w-3 shrink-0 text-xs font-bold text-ink-2">{text}</span>
{/snippet}

{#snippet ahead(n: number)}
	<span class="flex size-6 shrink-0 items-center justify-center rounded-full bg-well text-xs font-bold text-ink-2">{n}</span>
{/snippet}

<RestCard bind:this={rest} runId={openRun.run!.id} seconds={step.set_rest_seconds} />

{#if logged.length || !isFinished(step)}
	<div class="rounded-3xl bg-surface py-0.5 shadow-card">
		{#if done.length}
			<ol>
				{#each done as pair, i (i)}
					<li class="flex h-11 items-center gap-2.5 border-line px-4 text-[15px] [&:not(:first-child)]:border-t">
						{#if pair.length === sides}
							{@render tick()}
						{:else}
							{@render ahead(i + 1)}
						{/if}
						<span class="sr-only">Set {i + 1}</span>
						{#if sides === 2}
							{#each pair as s (s.number)}
								{@render sideLabel(side(s.number)[0])}
								<span class="min-w-0 flex-1 truncate font-bold">{short(s)}</span>
							{/each}
						{:else}
							<span class="font-bold">{pair[0].reps !== null ? `${pair[0].reps} reps` : pair[0].weight_kg === null ? 'Done' : ''}</span>
							<span class="flex-1 font-semibold text-ink-2">{pair[0].weight_kg !== null ? `${pair[0].weight_kg} kg` : ''}</span>
						{/if}
					</li>
				{/each}
			</ol>
		{/if}

		{#if !isFinished(step)}
			<section class="mx-1.5 my-0.5 rounded-[20px] bg-well p-2.5 dark:bg-[#1F2721]">
				{#if sides === 2}
					<div class="mb-2 grid grid-cols-2 gap-2">
						<p class="rounded-2xl bg-ink px-3.5 pt-2.5 pb-3 text-ground shadow-card-sm">
							<span class="block text-xs font-semibold">Set {number}{planned ? ` of ${planned / sides}` : ''} · now</span>
							<span class="block text-[32px] leading-[1.05] font-extrabold tracking-tight">{side(row)}</span>
						</p>
						<p class="rounded-2xl bg-ink/5 px-3.5 pt-2.5 pb-3 dark:bg-white/5">
							<span class="block text-xs font-semibold text-ink-2">Next</span>
							<span class="block text-[32px] leading-[1.05] font-extrabold tracking-tight text-ink-2">{side(row + 1)}</span>
						</p>
					</div>
				{:else}
					<p class="px-1 pb-2 text-[15px] font-bold">Set {number}{planned ? ` of ${planned}` : ''}</p>
				{/if}
				{#if lastTime}
					<p class="px-1 pb-2.5 text-xs font-semibold text-ink-2">
						Last time: {lastTime.sets.map(describeSet).filter(Boolean).join(', ')}
					</p>
				{/if}
				<div class="grid grid-cols-2 gap-2">
					<Stepper label="Reps" bind:value={reps} />
					<Stepper label="kg" step={2.5} min={-200} placeholder="–" bind:value={weight} />
				</div>
				<div class="mt-2.5">
					<Button onclick={log}>
						<svg viewBox="0 0 24 24" class="size-5" fill="none" stroke="currentColor" stroke-width="2.6" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M5 12.5l4.5 4.5L19 7" /></svg>
						Log {sides === 2 ? side(row).toLowerCase() : `set ${number}`}
					</Button>
				</div>
			</section>
		{/if}

		{#if todo.length}
			<ol class={sides === 2 ? '' : 'grid grid-cols-2'}>
				{#each todo as n (n)}
					<li class="flex h-11 items-center gap-2.5 px-4 text-[15px] font-bold text-ink-2">
						{@render ahead(n)}
						{#if sides === 2}
							{@render sideLabel('L')}
							<span class="flex-1">{step.reps !== null ? `${step.reps} reps` : ''}</span>
							{@render sideLabel('R')}
							<span class="flex-1">{step.reps !== null ? `${step.reps} reps` : ''}</span>
						{:else if step.reps !== null}
							<span>{step.reps} reps</span>
						{/if}
					</li>
				{/each}
			</ol>
		{/if}
	</div>
{/if}
