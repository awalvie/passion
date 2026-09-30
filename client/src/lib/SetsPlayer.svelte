<script lang="ts">
	import Button from './Button.svelte';
	import RestCard from './RestCard.svelte';
	import { isFinished, setsOf, type RunStep, type SetFields } from './run';
	import { openRun } from './runState.svelte';
	import Stepper from './Stepper.svelte';

	let { step }: { step: RunStep } = $props();

	const logged = $derived(setsOf(openRun.run!, step.id));
	const planned = $derived(step.sets ?? 0);
	const number = $derived(logged.length + 1);

	let reps = $state<number | null>(null);
	let weight = $state<number | null>(null);

	// A new step starts from its plan, and each set after that from the last one.
	$effect.pre(() => {
		const last = logged.at(-1);
		reps = last?.reps ?? step.reps;
		weight = last?.weight_kg ?? null;
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
		<p class="text-center text-base font-semibold">
			Set {number}{planned ? ` of ${planned}` : ''}
		</p>
		<div class="grid grid-cols-2 gap-2">
			<Stepper label="Reps" bind:value={reps} />
			<Stepper label="kg" step={2.5} min={-200} placeholder="–" bind:value={weight} />
		</div>
		<Button onclick={log}>Log set {number}</Button>
	</section>
{/if}
