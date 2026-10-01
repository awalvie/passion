<script lang="ts">
	import { untrack } from 'svelte';
	import { goto } from '$app/navigation';
	import { tone, unlock } from './audio';
	import Button from './Button.svelte';
	import Menu from './Menu.svelte';
	import SaveStatus from './SaveStatus.svelte';
	import { summary } from './exercise';
	import { isFinished, setsOf, type RunStep } from './run';
	import { openRun } from './runState.svelte';
	import Stepper from './Stepper.svelte';
	import TimerFlap from './TimerFlap.svelte';
	import { at, elapsed, endSet, jump, newClock, rows, startOf, timeline, togglePause, type Phase } from './timeline';
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

	async function endSession() {
		if (!timed) return;
		endExercise();
		await goto(`/run/${runId}/finish`);
	}

	let holding = $state(false);
	let held: ReturnType<typeof setTimeout> | undefined;

	function holdStart(e: PointerEvent) {
		if (e.button !== 0) return;
		clearTimeout(held);
		holding = true;
		held = setTimeout(endSession, 1000);
	}

	function holdStop() {
		holding = false;
		clearTimeout(held);
	}

	$effect(() => holdStop);

	const labels = { prep: 'Prep', hang: 'Hang', rest: 'Rest' };

	function span(ms: number) {
		const s = Math.round(ms / 1000);
		if (s < 60) return `${s} s`;
		return s % 60 ? `${Math.floor(s / 60)} min ${s % 60} s` : `${s / 60} min`;
	}

	function then(p: Phase) {
		if (p.set !== phase.set) return `set ${p.set}`;
		if (p.side !== phase.side) return `${p.side!.toLowerCase()} side`;
		return `rep ${p.rep}`;
	}

	const nextText = $derived.by(() => {
		if (!phase || !upcoming) return 'Last rep';
		if (upcoming.kind === 'rest') {
			const after = phases[pos.index + 2];
			return `Rest ${span(upcoming.ms)}${after ? `, then ${then(after)}` : ''}`;
		}
		if (phase.kind === 'prep') return `Hang ${span(upcoming.ms)}`;
		return `Then ${then(upcoming)}`;
	});

	// A darker glass than the mock so text passes 4.5:1 when read from the floor.
	const light = {
		ink: 'text-on-timer',
		ink2: 'text-on-timer/90',
		glass: 'bg-black/20',
		ring: 'text-on-timer/74',
		accent: ''
	};
	const looks = {
		prep: {
			...light,
			band: 'bg-prep-band',
			field: 'bg-prep',
			card: 'bg-prep-card',
			digit: 'text-prep-digit',
			split: 'bg-prep'
		},
		hang: {
			ink: 'text-on-hang',
			ink2: 'text-on-hang/70',
			glass: 'bg-on-hang/10',
			ring: 'text-on-hang/70',
			accent: '',
			band: 'bg-hang-band',
			field: 'bg-hang',
			card: 'bg-on-hang',
			digit: 'text-hang-digit',
			split: 'bg-hang'
		},
		rest: {
			ink: 'text-ink',
			ink2: 'text-ink/65',
			glass: 'bg-ink/7 dark:bg-ink/8',
			ring: 'text-ink/65',
			accent: 'text-rest-ink',
			band: 'bg-rest-soft',
			field: 'bg-ground',
			card: 'bg-rest-card',
			digit: 'text-rest-digit',
			split: 'bg-ground'
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
			class="absolute inset-x-0 bottom-0 shadow-[0_-2px_0_rgba(0,0,0,0.08)] {look.band}"
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

			<p class="flex items-center gap-3 px-6 pt-2 {look.accent} font-[family-name:var(--font-digits)] text-[52px] leading-none font-bold tracking-[0.04em] uppercase">
				<svg width="34" height="34" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.6" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
					{#if phase.kind === 'prep'}
						<path d="M6.5 3.5h11M6.5 20.5h11" /><path d="M8 3.5c0 5 8 5 8 8.5s-8 3.5-8 8.5M16 3.5c0 5-8 5-8 8.5s8 3.5 8 8.5" />
					{:else if phase.kind === 'hang'}
						<path d="M3 4h18" /><path d="M7.5 4l3 6.5M16.5 4l-3 6.5" /><circle cx="12" cy="10.2" r="2.2" /><path d="M12 12.6v4.4M12 17l-2.8 4M12 17l2.8 4" />
					{:else}
						<path d="M2.5 13c2.2-6 4.4-6 6.6 0s4.4 6 6.6 0 3.5-4.5 5.8-2" />
					{/if}
				</svg>
				{labels[phase.kind]}
			</p>
			<p class="px-6 pt-1 text-[15px] font-semibold {look.ink2}">
				{[
					step.name,
					phase.side,
					setOver ? `set ${setsDone} of ${step.sets} done` : weight ? `${weight > 0 ? '+' : ''}${weight} kg` : null,
					paused ? 'Paused' : null
				]
					.filter(Boolean)
					.join(' · ')}
			</p>

			<div
				class="mt-3.5 flex shrink-0 items-center justify-center gap-2 {clock.length === 1 ? '[--h:min(300px,36svh,76vw)]' : '[--h:min(236px,28svh,56vw)]'}"
				role="img"
				aria-live="off"
				aria-label="{secs} {secs === 1 ? 'second' : 'seconds'} left"
			>
				{#each clock.split('') as digit, i (i)}
					{#if secs >= 60 && i === clock.length - 2}
						<div class="flex w-4 flex-col items-center gap-[calc(var(--h)*0.127)]" aria-hidden="true">
							<i class="size-4 rounded-full {look.card}"></i>
							<i class="size-4 rounded-full {look.card}"></i>
						</div>
					{/if}
					<TimerFlap {digit} wide={clock.length === 1} {look} />
				{/each}
				{#if secs < 60}
					<span class="ml-1.5 self-end pb-[22px] font-[family-name:var(--font-digits)] text-[44px] leading-none font-bold">s</span>
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

			<div class="mx-4 mt-[18px] flex h-14 items-center gap-2 rounded-full pr-1.5 pl-3.5 text-xl font-bold tracking-[-0.01em] {look.glass}">
				<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.6" stroke-linecap="round" stroke-linejoin="round" class="shrink-0" aria-hidden="true"><path d="M9 6l6 6-6 6" /></svg>
				<span class="min-w-0 flex-1 truncate">{nextText}</span>
				<button
					type="button"
					class="flex h-11 shrink-0 items-center gap-1.5 rounded-full px-3.5 text-[15px] font-bold {look.glass}"
					onpointerdown={unlock}
					onclick={skipPhase}
				>
					<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M6 5.5v13l9-6.5z" fill="currentColor" /><path d="M18.5 5v14" /></svg>
					Skip
				</button>
			</div>

			<div class="mt-auto flex gap-2.5 px-4 pt-4" onpointerdown={unlock} role="group" aria-label="Timer">
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
					class="flex h-17 touch-none items-center gap-2.5 rounded-full pr-5 pl-3 text-left select-none [-webkit-touch-callout:none] {look.glass}"
					onpointerdown={holdStart}
					onpointerup={holdStop}
					onpointercancel={holdStop}
					onpointerleave={holdStop}
					oncontextmenu={(e) => e.preventDefault()}
					onclick={(e) => e.detail === 0 && confirm('End the session?') && endSession()}
				>
					<span class="relative flex size-11 items-center justify-center rounded-full shadow-[inset_0_0_0_3px] {look.ring}">
						<svg viewBox="0 0 44 44" class="absolute inset-0 -rotate-90" aria-hidden="true">
							<circle
								cx="22"
								cy="22"
								r="20.5"
								fill="none"
								stroke="currentColor"
								stroke-width="3"
								stroke-dasharray="129"
								stroke-dashoffset={holding ? 0 : 129}
								class="{look.ink} {holding ? 'transition-[stroke-dashoffset] duration-1000 ease-linear' : ''}"
							/>
						</svg>
						<i class="size-[9px] rounded-[2px] bg-current {look.ink}" aria-hidden="true"></i>
					</span>
					<span class="flex flex-col leading-[1.15]">
						<b class="text-[15px] font-bold">Hold to end</b>
						<small class="text-xs font-semibold {look.ink2}">the session</small>
					</span>
				</button>
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
