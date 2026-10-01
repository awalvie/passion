<script lang="ts">
	import Icon from './Icon.svelte';
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
		timed = stored?.step === step.id && !isFinished(step) ? stored : null;
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
	<section class="rounded-3xl bg-surface px-5 pt-4 pb-5 shadow-card">
		<p class="flex items-center gap-2.5 text-xs font-bold tracking-[0.06em] text-ink-2 uppercase">
			<i class="size-2.5 rounded-full {timed.clock.pausedAt === null ? 'bg-live shadow-[0_0_0_5px_var(--live-halo)]' : 'bg-ink-3'}"></i>
			{target ? 'Left' : 'Time'}
		</p>
		<p class="mt-2 text-[64px] leading-none font-extrabold tracking-tighter">
			{formatClock(target ? (target - ms) / 1000 : Math.floor(ms / 1000))}
		</p>
		{#if target}
			<div class="mt-4 h-1.5 overflow-hidden rounded-full bg-well">
				<i class="block h-full rounded-full bg-live" style="width: {Math.min(100, (ms / target) * 100)}%"></i>
			</div>
		{/if}
	</section>
	<div class="grid grid-cols-2 gap-2.5">
		<Button variant="secondary" onclick={pause}>{timed.clock.pausedAt === null ? 'Pause' : 'Resume'}</Button>
		<Button onclick={() => done(ms)}>Done</Button>
	</div>
{:else if !isFinished(step)}
	<section class="flex flex-col gap-4 rounded-3xl bg-surface p-4 shadow-card">
		<p class="text-center text-[32px] leading-[1.1] font-extrabold tracking-tight">
			{#if target}{formatClock(target / 1000)}{:else}<span class="text-xl font-bold">As long as it takes</span>{/if}
		</p>
		<Button onclick={start}>Start</Button>
	</section>
{:else if logged[0]?.seconds}
	<p class="flex h-11 items-center gap-2.5 rounded-3xl bg-surface px-4 text-[15px] font-bold shadow-card">
		<span class="flex size-6 shrink-0 items-center justify-center rounded-full bg-ink text-ground dark:bg-ink-2">
			<Icon name="check" size="0.75rem" stroke={3.2} />
		</span>
		Took {formatClock(logged[0].seconds)}
	</p>
{/if}
