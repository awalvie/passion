<script lang="ts">
	import { request } from './api';
	import Button from './Button.svelte';
	import RestCard from './RestCard.svelte';
	import { isFinished, setsOf, type HistorySession, type RunStep, type SetFields } from './run';
	import { openRun } from './runState.svelte';
	import Stepper from './Stepper.svelte';

	let { step }: { step: RunStep } = $props();

	const logged = $derived(setsOf(openRun.run!, step.id));
	const planned = $derived(step.sets ?? 0);
	const number = $derived(logged.length + 1);

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
				if (current) lastTime = h.sessions.find((s) => s.sets.length) ?? null;
			})
			.catch(() => {});
		return () => (current = false);
	});

	// Each set starts from the one before it, the first from the same set last
	// time, and else from the plan.
	$effect.pre(() => {
		const before = logged.at(-1);
		const then = lastTime?.sets[logged.length] ?? lastTime?.sets.at(-1);
		reps = before?.reps ?? then?.reps ?? step.reps;
		weight = before?.weight_kg ?? then?.weight_kg ?? null;
	});

	let rest: ReturnType<typeof RestCard>;

	function log() {
		const sets: SetFields[] = logged.map(({ reps, seconds, weight_kg }) => ({ reps, seconds, weight_kg }));
		openRun.setSets(step, [...sets, { reps, seconds: null, weight_kg: weight }]);
		if (planned && sets.length + 1 >= planned) openRun.finish(step);
		else rest.start();
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
				<span class="text-ink-2">Set {s.number}</span>
				<span class="font-semibold tabular-nums">{describeSet(s) || 'Done'}</span>
			</li>
		{/each}
	</ol>
{/if}

{#if !isFinished(step)}
	<section class="flex flex-col gap-4 rounded-2xl bg-surface p-4 shadow-sm">
		<div class="text-center">
			<p class="text-base font-semibold">Set {number}{planned ? ` of ${planned}` : ''}</p>
			{#if lastTime}
				<p class="text-sm text-ink-2">
					Last time: {lastTime.sets.map(describeSet).filter(Boolean).join(', ')}
				</p>
			{/if}
		</div>
		<div class="grid grid-cols-2 gap-2">
			<Stepper label="Reps" bind:value={reps} />
			<Stepper label="kg" step={2.5} min={-200} placeholder="–" bind:value={weight} />
		</div>
		<Button onclick={log}>Log set {number}</Button>
	</section>
{/if}
