<script lang="ts">
	import { untrack } from 'svelte';
	import { tone, unlock } from './audio';
	import Button from './Button.svelte';
	import Menu from './Menu.svelte';
	import SaveStatus from './SaveStatus.svelte';
	import { summary } from './exercise';
	import { isFinished, setsOf, type RunStep } from './run';
	import { openRun } from './runState.svelte';
	import Stepper from './Stepper.svelte';
	import TimerFlap from './TimerFlap.svelte';
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

	const labels = { prep: 'Prep', hang: 'Hang', rest: 'Rest' };
	const skipLabels = { prep: 'Skip prep', hang: 'End rep', rest: 'Skip rest' };

	// Darker fields and glass than the tokens so text passes 4.5:1 when read from the floor.
	const light = { ink: 'text-[#F6F9F4]', ink2: 'text-[#F6F9F4]/90', glass: 'bg-black/20' };
	const looks = {
		prep: {
			...light,
			field: 'bg-[#1a50b8] dark:bg-prep',
			card: 'bg-[#0D2A66] dark:bg-[#091E4B]',
			digit: 'text-[#F3F7FF] dark:text-[#E8EFFF]',
			split: 'bg-[#1a50b8] dark:bg-prep'
		},
		hang: {
			ink: 'text-on-hang',
			ink2: 'text-on-hang/70',
			glass: 'bg-on-hang/10',
			field: 'bg-hang',
			card: 'bg-on-hang',
			digit: 'text-[#C6F05B] dark:text-[#BFEA55]',
			split: 'bg-hang'
		},
		rest: {
			...light,
			field: 'bg-[#095c37] dark:bg-[#0A6A3F]',
			card: 'bg-[#04311D] dark:bg-[#032717]',
			digit: 'text-[#EAF7EE] dark:text-[#DDF2E4]',
			split: 'bg-[#095c37] dark:bg-[#0A6A3F]'
		}
	};

	const look = $derived(looks[phase?.kind ?? 'prep']);
	const repsDone = $derived(phase ? (phase.kind === 'rest' ? phase.rep : phase.rep - 1) : 0);
	const setOver = $derived(phase?.kind === 'rest' && phase.rep === step.reps && upcoming?.set !== phase.set);
	const setsDone = $derived(phase ? (setOver ? phase.set : phase.set - 1) : 0);

	function dash(i: number, done: number) {
		if (i < done) return 'bg-current';
		if (i === done) return 'bg-current opacity-45';
		return look.glass;
	}

</script>

{#if timed && phase}
	{@const secs = Math.ceil(pos.left / 1000)}
	{@const clock = secs >= 60 ? `${Math.floor(secs / 60)}${String(secs % 60).padStart(2, '0')}` : String(secs)}
	{@const paused = timed.clock.pausedAt !== null}
	<section
		class="fixed inset-0 z-40 overflow-hidden {look.field} {look.ink}"
		aria-label="{labels[phase.kind]} timer"
	>
		<div
			class="absolute inset-x-0 bottom-0 bg-black/15 shadow-[0_-2px_0_rgba(0,0,0,0.08)]"
			style="height: {(pos.left / phase.ms) * 100}%"
			aria-hidden="true"
		></div>

		<div
			class="relative mx-auto flex h-full w-full max-w-[430px] flex-col pt-[env(safe-area-inset-top)] pb-[calc(env(safe-area-inset-bottom)+1rem)]"
		>
			<div class="flex h-14 shrink-0 items-center justify-between px-4">
				<a
					href="/run/{runId}"
					class="inline-flex h-11 items-center gap-0.5 rounded-full pr-4 pl-2.5 text-[15px] font-bold {look.glass}"
				>
					<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M15 18l-6-6 6-6" /></svg>
					Session
				</a>
				<Menu
					look={look.glass}
					items={[
						{ label: 'End set', onclick: cutSet },
						{ label: 'End exercise', onclick: endExercise }
					]}
				/>
			</div>
			<div class="flex flex-col gap-2 px-4 empty:hidden"><SaveStatus /></div>

			<p class="flex items-center gap-3.5 px-6 pt-4 text-[32px] leading-none font-extrabold tracking-[0.1em] uppercase">
				{#if phase.kind === 'prep'}
					<i class="h-0 w-0 border-x-[13px] border-b-[22px] border-x-transparent border-b-current" aria-hidden="true"></i>
				{:else if phase.kind === 'hang'}
					<i class="size-5.5 rounded-md bg-current" aria-hidden="true"></i>
				{:else}
					<i class="size-5.5 rounded-full border-4 border-current" aria-hidden="true"></i>
				{/if}
				{labels[phase.kind]}
			</p>
			<p class="px-6 pt-2 text-[15px] font-semibold {look.ink2}">
				{step.name}{phase.side ? ` · ${phase.side}` : ''}{weight ? ` · ${weight > 0 ? '+' : ''}${weight} kg` : ''}
				{#if paused}· Paused{/if}
			</p>

			<div
				class="mt-[min(34px,4svh)] flex shrink-0 items-center justify-center gap-2 [--h:min(232px,28svh,56vw)]"
				role="img"
				aria-live="off"
				aria-label="{secs} seconds left"
			>
				{#each clock.split('') as digit, i (i)}
					{#if secs >= 60 && i === clock.length - 2}
						<div class="flex w-4.5 flex-col items-center gap-[calc(var(--h)*0.15)]" aria-hidden="true">
							<i class="size-4 rounded-full {look.card}"></i>
							<i class="size-4 rounded-full {look.card}"></i>
						</div>
					{/if}
					<TimerFlap {digit} wide={clock.length === 1} {look} />
				{/each}
				{#if secs < 60}
					<span class="ml-1.5 self-end pb-4.5 text-[44px] leading-none font-bold">s</span>
				{/if}
			</div>

			<div class="flex flex-col gap-3.5 px-6 pt-[min(34px,4svh)]">
				<div class="flex items-center justify-between gap-4">
					<span class="text-xl font-semibold {look.ink2}">
						Set <b class="mx-px text-[32px] font-extrabold {look.ink}">{phase.set}</b>/{step.sets}
					</span>
					<span class="flex w-[210px] gap-1.5" aria-hidden="true">
						{#each { length: step.sets ?? 0 }, i (i)}
							<i class="h-2 flex-1 rounded-full {dash(i, setsDone)}"></i>
						{/each}
					</span>
				</div>
				<div class="flex items-center justify-between gap-4">
					<span class="text-xl font-semibold {look.ink2}">
						Rep <b class="mx-px text-[32px] font-extrabold {look.ink}">{phase.rep}</b>/{step.reps}
					</span>
					<span class="flex w-[210px] gap-1.5" aria-hidden="true">
						{#each { length: step.reps ?? 0 }, i (i)}
							<i class="h-2 flex-1 rounded-full {dash(i, repsDone)}"></i>
						{/each}
					</span>
				</div>
			</div>

			{#if upcoming}
				<p class="mx-6 mt-5 flex items-center gap-2 text-xl font-bold">
					<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.6" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M9 6l6 6-6 6" /></svg>
					Next: {labels[upcoming.kind]} {upcoming.ms / 1000} s
				</p>
			{/if}

			<div class="mt-auto flex flex-col gap-2.5 px-4 pt-4" onpointerdown={unlock} role="group" aria-label="Timer">
				<div class="flex gap-2.5">
					<button
						type="button"
						class="flex h-17 flex-1 items-center justify-center gap-2.5 rounded-full px-6 text-xl font-bold {look.glass}"
						onclick={pause}
					>
						{#if paused}
							<svg width="20" height="20" viewBox="0 0 24 24" aria-hidden="true"><path d="M7 4.5v15l12-7.5z" fill="currentColor" /></svg>
							Resume
						{:else}
							<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round" aria-hidden="true"><path d="M8 5v14M16 5v14" /></svg>
							Pause
						{/if}
					</button>
					<button
						type="button"
						class="flex h-17 items-center justify-center gap-2.5 rounded-full px-6 text-xl font-bold whitespace-nowrap {look.glass}"
						onclick={skipPhase}
					>
						<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M6 5.5v13l9-6.5z" fill="currentColor" /><path d="M18.5 5v14" /></svg>
						{skipLabels[phase.kind]}
					</button>
				</div>
			</div>
		</div>
	</section>
{:else if !isFinished(step)}
	<section class="flex flex-col gap-4 rounded-3xl bg-surface p-5 shadow-card">
		<p class="text-center text-[15px] font-bold">
			{logged.length ? `${logged.length} of ${blocks} logged` : summary(step)}
		</p>
		<Stepper label="kg" step={2.5} min={-200} placeholder="–" bind:value={weight} />
		<Button onclick={start}>{logged.length ? 'Continue' : 'Start'}</Button>
	</section>
{/if}
