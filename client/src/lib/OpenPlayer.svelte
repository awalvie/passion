<script lang="ts">
	import { tone, unlock } from './audio';
	import Button from './Button.svelte';
	import { isFinished, setsOf, type RunStep } from './run';
	import { openRun } from './runState.svelte';
	import { elapsed, newClock, togglePause } from './timeline';
	import { formatClock, readTimers, writeTimers, type Timed } from './timerStore';

	let { step }: { step: RunStep } = $props();

	const runId = $derived(openRun.run!.id);
	const logged = $derived(setsOf(openRun.run!, step.id));
	const target = $derived((step.duration_seconds ?? 0) * 1000);

	let timed = $state<Timed | null>(null);
	let now = $state(Date.now());

	$effect.pre(() => {
		const stored = readTimers(runId).timed;
		timed = stored?.step === step.id ? stored : null;
	});

	$effect(() => {
		if (!timed || timed.clock.pausedAt !== null) return;
		const tick = setInterval(() => (now = Date.now()), 250);
		return () => clearInterval(tick);
	});

	const ms = $derived(timed ? elapsed(timed.clock, now) : 0);

	// A countdown logs its full time and ends by itself.
	$effect(() => {
		if (timed && target && ms >= target) {
			if (ms - target < 2000) tone(1046, 400);
			done(target);
		}
	});

	function save(next: Timed | null) {
		timed = next;
		now = Date.now();
		writeTimers(runId, { ...readTimers(runId), timed: next });
	}

	function start() {
		unlock();
		save({ step: step.id, clock: newClock(Date.now()), short: {}, weight: null });
	}

	function pause() {
		save({ ...timed!, clock: togglePause(timed!.clock, Date.now()) });
	}

	function done(spent: number) {
		save(null);
		openRun.setSets(step, [{ reps: null, seconds: Math.round(spent / 1000), weight_kg: null }]);
		openRun.finish(step);
	}
</script>

{#if timed}
	<section class="flex flex-col items-center gap-2 rounded-3xl bg-prep py-8 text-on-tint shadow-sm">
		<p class="text-xl font-semibold">{target ? 'Left' : 'Time'}</p>
		<p class="text-7xl font-bold tabular-nums">{formatClock(target ? (target - ms) / 1000 : Math.floor(ms / 1000))}</p>
	</section>
	<div class="grid grid-cols-2 gap-2">
		<Button variant="secondary" onclick={pause}>{timed.clock.pausedAt === null ? 'Pause' : 'Resume'}</Button>
		<Button onclick={() => done(ms)}>Done</Button>
	</div>
{:else if !isFinished(step)}
	<section class="flex flex-col gap-4 rounded-2xl bg-surface p-4 shadow-sm">
		<p class="text-center text-base font-semibold">
			{target ? formatClock(target / 1000) : 'As long as it takes'}
		</p>
		<Button onclick={start}>Start</Button>
	</section>
{:else if logged[0]?.seconds}
	<p class="rounded-2xl bg-surface px-4 py-3 text-base shadow-sm">Took {formatClock(logged[0].seconds)}</p>
{/if}
