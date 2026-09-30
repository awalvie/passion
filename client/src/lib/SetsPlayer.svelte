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

	function describeSet(s: SetFields) {
		return [s.reps !== null && `${s.reps} reps`, s.weight_kg !== null && `${s.weight_kg} kg`]
			.filter(Boolean)
			.join(' · ');
	}
</script>

<RestCard bind:this={rest} runId={openRun.run!.id} seconds={step.set_rest_seconds} />

{#if logged.length}
	<ol class="overflow-hidden rounded-2xl bg-surface shadow-sm">
		{#each logged as s (s.number)}
			<li class="flex items-center justify-between border-line px-4 py-3 text-base [&:not(:first-child)]:border-t">
				<span class="text-ink-2">
					Set {Math.ceil(s.number / sides)}{sides === 2 ? ` · ${side(s.number)}` : ''}
				</span>
				<span class="font-semibold tabular-nums">{describeSet(s) || 'Done'}</span>
			</li>
		{/each}
	</ol>
{/if}

{#if !isFinished(step)}
	<section class="flex flex-col gap-4 rounded-2xl bg-surface p-4 shadow-sm">
		<div class="text-center">
			<p class="text-base font-semibold">Set {number}{planned ? ` of ${planned / sides}` : ''}</p>
			{#if lastTime}
				<p class="text-sm text-ink-2">
					Last time: {lastTime.sets.map(describeSet).filter(Boolean).join(', ')}
				</p>
			{/if}
		</div>
		{#if sides === 2}
			<div class="grid grid-cols-2 gap-2 text-center">
				<p class="rounded-xl bg-tint py-3 text-on-tint">
					<span class="block text-sm opacity-90">Now</span>
					<span class="block text-xl font-semibold">{side(row)}</span>
				</p>
				<p class="rounded-xl bg-ground py-3 text-ink-2">
					<span class="block text-sm">Next</span>
					<span class="block text-xl font-semibold">{side(row + 1)}</span>
				</p>
			</div>
		{/if}
		<div class="grid grid-cols-2 gap-2">
			<Stepper label="Reps" bind:value={reps} />
			<Stepper label="kg" step={2.5} min={-200} placeholder="–" bind:value={weight} />
		</div>
		<Button onclick={log}>Log {sides === 2 ? side(row).toLowerCase() : `set ${number}`}</Button>
	</section>
{/if}
