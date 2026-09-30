<script lang="ts">
	import { untrack } from 'svelte';
	import { tone, unlock } from './audio';
	import Button from './Button.svelte';
	import { summary } from './exercise';
	import { isFinished, setsOf, type RunStep } from './run';
	import { openRun } from './runState.svelte';
	import Stepper from './Stepper.svelte';
	import { at, elapsed, endSet, jump, newClock, rows, startOf, timeline, togglePause } from './timeline';
	import { readTimers, writeTimers, type Timed } from './timerStore';

	let { step }: { step: RunStep } = $props();

	const runId = $derived(openRun.run!.id);
	const phases = $derived(timeline(step));
	const logged = $derived(setsOf(openRun.run!, step.id));

	let timed = $state<Timed | null>(null);
	let weight = $state<number | null>(null);
	let now = $state(Date.now());

	$effect.pre(() => {
		const stored = readTimers(runId).timed;
		const mine = stored?.step === step.id && !isFinished(step) ? stored : null;
		timed = mine;
		weight = mine?.weight ?? untrack(() => logged.at(-1)?.weight_kg) ?? null;
	});

	$effect(() => {
		if (!timed || timed.clock.pausedAt !== null) return;
		const tick = setInterval(() => (now = Date.now()), 200);
		return () => clearInterval(tick);
	});

	const ms = $derived(timed ? elapsed(timed.clock, now) : 0);
	const pos = $derived(at(phases, ms));
	const phase = $derived(phases[pos.index]);
	const upcoming = $derived(phases[pos.index + 1]);
	const blocks = $derived(new Set(phases.map((p) => p.block)).size);

	// Each block logs a row once its hangs are over, even if the phone slept
	// through it.
	$effect(() => {
		if (!timed) return;
		const done = rows(phases, ms, timed.short);
		if (done.length > logged.length) log(done);
		if (pos.index >= phases.length) stop();
	});

	// A tick in each of the last three seconds of a phase, and a high tone as
	// a hang starts. A key per sound plays each one once.
	let played = '';
	$effect(() => {
		if (!timed || !phase || timed.clock.pausedAt !== null) return;
		const secs = Math.ceil(pos.left / 1000);
		const key = `${pos.index}:${secs}`;
		if (key === played) return;
		const fresh = phase.ms - pos.left < 1000;
		if (fresh && phase.kind === 'hang') tone(1046, 300);
		else if (fresh) tone(523, 200);
		else if (secs <= 3) tone(784, 80);
		played = key;
	});

	function save(next: Timed | null) {
		timed = next;
		now = Date.now();
		writeTimers(runId, { ...readTimers(runId), timed: next });
	}

	function log(done: number[]) {
		openRun.setSets(
			step,
			done.map((reps) => ({ reps, seconds: step.rep_seconds, weight_kg: weight }))
		);
	}

	function stop() {
		save(null);
		openRun.finish(step);
	}

	// A step with rows already logged starts at the block after them, and
	// keeps the reps those rows logged.
	function start() {
		unlock();
		const t = Date.now();
		let clock = newClock(t);
		const first = phases.findIndex((p) => p.block === logged.length && p.kind === 'hang');
		if (logged.length && first > 0) clock = jump(clock, t, startOf(phases, first));
		const short = Object.fromEntries(logged.map((s, i) => [i, s.reps ?? 0]));
		save({ step: step.id, clock, short, weight });
	}

	function pause() {
		save({ ...timed!, clock: togglePause(timed!.clock, Date.now()) });
	}

	function skipPhase() {
		const t = Date.now();
		save({ ...timed!, clock: jump(timed!.clock, t, startOf(phases, pos.index + 1)) });
	}

	function cutSet() {
		const { clock, short } = endSet(phases, timed!.clock, Date.now(), timed!.short);
		save({ ...timed!, clock, short });
	}

	function endExercise() {
		const done = rows(phases, ms, timed!.short, true);
		if (done.length > logged.length) log(done);
		stop();
	}

	const colours = { prep: 'bg-prep', hang: 'bg-hang', rest: 'bg-rest' };
	const labels = { prep: 'Prep', hang: 'Hang', rest: 'Rest' };
	const skipLabels = { prep: 'Skip prep', hang: 'End rep', rest: 'Skip rest' };
</script>

{#if timed && phase}
	{@const secs = Math.ceil(pos.left / 1000)}
	<section class="relative flex flex-col items-center gap-2 overflow-hidden rounded-3xl py-8 text-on-tint shadow-sm {colours[phase.kind]}">
		<div
			class="absolute inset-x-0 bottom-0 bg-black/15"
			style="height: {(pos.left / phase.ms) * 100}%"
			aria-hidden="true"
		></div>
		<p class="relative text-xl font-semibold">{labels[phase.kind]}</p>
		<p class="relative text-[7rem] leading-none font-bold tabular-nums" aria-live="off">{secs}</p>
		<p class="relative text-base">
			Rep {phase.rep} of {step.reps} · Set {phase.set} of {step.sets}{phase.side ? ` · ${phase.side}` : ''}
		</p>
		{#if upcoming}
			<p class="relative text-sm opacity-90">Next: {labels[upcoming.kind]} {upcoming.ms / 1000} s</p>
		{/if}
	</section>

	<div class="grid grid-cols-2 gap-2" onpointerdown={unlock} role="group" aria-label="Timer">
		<Button variant="secondary" onclick={pause}>{timed.clock.pausedAt === null ? 'Pause' : 'Resume'}</Button>
		<Button variant="secondary" onclick={skipPhase}>{skipLabels[phase.kind]}</Button>
		<Button variant="secondary" onclick={cutSet}>End set</Button>
		<Button variant="secondary" onclick={endExercise}>End exercise</Button>
	</div>
{:else if !isFinished(step)}
	<section class="flex flex-col gap-4 rounded-2xl bg-surface p-4 shadow-sm">
		<p class="text-center text-base font-semibold">
			{logged.length ? `${logged.length} of ${blocks} logged` : summary(step)}
		</p>
		<Stepper label="kg" step={2.5} min={-200} placeholder="–" bind:value={weight} />
		<Button onclick={start}>{logged.length ? 'Continue' : 'Start'}</Button>
	</section>
{/if}
