<script lang="ts">
	import Icon from './Icon.svelte';
	import { tone } from './audio';
	import type { MenuItem } from './Menu.svelte';
	import { isFinished, setsOf, type RunStep } from './run';
	import { openRun } from './runState.svelte';
	import RunPage from './RunPage.svelte';
	import type { NextLine } from './template';
	import { plainText } from './text';
	import { elapsed, newClock, togglePause } from './timeline';
	import { formatClock, readTimers, writeTimers, type Timed } from './timerStore';

	let {
		step,
		next,
		nextHref,
		last,
		menu,
		error,
		leaving,
		leave
	}: {
		step: RunStep;
		next: NextLine;
		nextHref: string;
		last: boolean;
		menu: MenuItem[];
		error: string;
		leaving: boolean;
		leave: (end: () => void) => Promise<void>;
	} = $props();

	const run = $derived(openRun.run!);
	const runId = $derived(run.id);
	const logged = $derived(setsOf(run, step.id));
	const target = $derived((step.duration_seconds ?? 0) * 1000);
	const howTo = $derived(plainText(step.notes ?? '').trim());
	const lines = $derived(
		howTo
			.split('\n')
			.map((l) => l.trim())
			.filter(Boolean)
	);

	let reading = $state(false);
	let box = $state<HTMLElement>();
	let over = $state(false);
	$effect(() => {
		void lines;
		if (box) over = box.scrollHeight > box.clientHeight + 1;
	});
	let timed = $state<Timed | null>(null);
	let now = $state(Date.now());

	// An open step starts its clock as soon as it opens, unless another step's
	// clock is still running: there is one clock, and its time is not logged yet.
	$effect.pre(() => {
		const stored = readTimers(runId).timed;
		const other = stored && stored.step !== step.id ? openRun.step(stored.step) : undefined;
		if (isFinished(step)) timed = null;
		else if (stored?.step === step.id) timed = stored;
		else if (other && !isFinished(other)) timed = null;
		else start();
	});

	const paused = $derived(timed?.clock.pausedAt != null);

	$effect(() => {
		const tick = setInterval(() => (now = Date.now()), timed && !paused ? 250 : 1000);
		return () => clearInterval(tick);
	});

	const ms = $derived(timed ? elapsed(timed.clock, now) : 0);

	// A countdown logs its full time and ends by itself.
	$effect(() => {
		if (timed && target && ms >= target) {
			if (ms - target < 2000) {
				tone(1046, 400);
				navigator.vibrate?.(400);
			}
			done(target);
		}
	});

	function save(next: Timed | null) {
		timed = next;
		now = Date.now();
		writeTimers(runId, { ...readTimers(runId), timed: next });
	}

	function start() {
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

	function finish() {
		void leave(() => {
			if (timed && ms >= 1000) done(ms);
			else {
				if (timed) save(null);
				openRun.finish(step);
			}
		});
	}

	function skip() {
		void leave(() => {
			if (timed) save(null);
			openRun.skip(step);
		});
	}

	const finished = $derived(isFinished(step) && !leaving);
	const took = $derived(logged[0]?.seconds ?? 0);
	const clock = $derived(
		finished ? formatClock(took) : !timed ? formatClock(target / 1000) : target ? formatClock((target - ms) / 1000) : formatClock(Math.floor(ms / 1000))
	);
	const clockSize = $derived(clock.length <= 4 ? 'text-[min(190px,48vw)]' : clock.length === 5 ? 'text-[min(150px,38vw)]' : 'text-[min(112px,28vw)]');
	const share = $derived(target ? Math.min(1, ms / target) : 0);
</script>

<RunPage
	{step}
	icon={finished ? 'check' : paused ? 'pause' : 'hourglass'}
	status={finished ? 'Done' : !timed ? 'Ready' : paused ? 'Paused' : 'Running'}
	{next}
	{menu}
	{error}
	bind:reading
>
	<div class="mt-10 flex flex-col">
		<p
			class="text-center font-[family-name:var(--font-digits)] leading-none font-bold tracking-[-0.02em] whitespace-nowrap {clockSize} {paused ? 'opacity-55' : ''}"
			role="timer"
			aria-live="off"
		>
			{clock}
		</p>
		{#if finished}
			<p class="mt-3 text-center text-[17px] font-bold text-(--fg2)">{took ? 'Time logged' : 'Done'}</p>
		{:else if target}
			<div class="mx-5 mt-6">
				<div class="h-3.5 overflow-hidden rounded-full bg-(--glass)">
					<i class="block h-full rounded-full bg-tint" style="width: {share * 100}%"></i>
				</div>
				<p class="mt-2.5 flex justify-between font-[family-name:var(--font-digits)] text-lg font-bold tracking-[0.04em] text-(--fg2)">
					<span><b class="text-(--fg)">{formatClock(ms / 1000)}</b> DONE</span>
					<span>OF {formatClock(target / 1000)}</span>
				</p>
			</div>
		{/if}
		{#if howTo && !finished}
			<button type="button" class="mx-6 mt-8 flex gap-3.5 text-left" onclick={() => (reading = true)}>
				<i class="w-[3px] shrink-0 rounded-full bg-tint"></i>
				<span class="min-w-0">
					<span bind:this={box} class="block max-h-[4.2em] overflow-hidden text-[17px] leading-[1.4] font-semibold whitespace-pre-line text-(--fg) {over ? 'fade' : ''}">{lines.join('\n')}</span>
					<span class="mt-2 flex items-center gap-1 text-[14px] font-bold text-(--fg2)">Read all<Icon name="chevron-right" size="15px" stroke={2.6} /></span>
				</span>
			</button>
		{/if}
	</div>

	{#snippet buttons()}
		{#if finished}
			<a href={nextHref} class="run-btn bg-live text-on-live">
				{last ? 'Back to session' : 'Next exercise'}
				<span class="run-icon"><Icon name="arrow-right" size="16px" stroke={2.6} /></span>
			</a>
		{:else if !timed}
			<button type="button" class="run-btn bg-tint text-on-tint" onclick={start}>
				<span class="run-icon"><Icon name="play" size="16px" /></span>Start
			</button>
			<button type="button" class="run-round" aria-label="Skip" onclick={skip}><Icon name="skip" size="26px" /></button>
		{:else}
			<button type="button" class="run-round {paused ? 'bg-tint text-on-tint' : 'bg-(--fg) text-(--field)'}" aria-label={paused ? 'Resume' : 'Pause'} onclick={pause}>
				<Icon name={paused ? 'play' : 'pause'} size="26px" stroke={3} />
			</button>
			<button type="button" class="run-btn {paused ? 'bg-(--glass)' : 'bg-tint text-on-tint'}" onclick={finish}>
				<span class="run-icon"><Icon name="check" size="16px" stroke={3} /></span>Done
			</button>
			<button type="button" class="run-round" aria-label="Skip" onclick={skip}><Icon name="skip" size="26px" /></button>
		{/if}
	{/snippet}
</RunPage>

<style>
	/* The last line fades out when the notes run on. */
	.fade {
		mask-image: linear-gradient(#000 calc(100% - 1.4em), transparent);
	}
</style>
