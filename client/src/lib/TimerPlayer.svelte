<script lang="ts">
	import { untrack } from 'svelte';
	import { goto } from '$app/navigation';
	import { tone, unlock } from './audio';
	import Icon from './Icon.svelte';
	import type { MenuItem } from './Menu.svelte';
	import { isFinished, nextStop, setsOf, stepsOf, type RunStep } from './run';
	import { openRun } from './runState.svelte';
	import RunPage from './RunPage.svelte';
	import Stepper from './Stepper.svelte';
	import TimerFlap from './TimerFlap.svelte';
	import { at, canTime, elapsed, endSet, jump, newClock, rows, startOf, timeline, togglePause, type Phase } from './timeline';
	import { readTimers, writeTimers, type Timed } from './timerStore';

	let {
		step,
		next,
		nextHref,
		last,
		menu,
		error,
		leaving,
		skip
	}: {
		step: RunStep;
		next: string;
		nextHref: string;
		last: boolean;
		menu: MenuItem[];
		error: string;
		leaving: boolean;
		skip: () => void;
	} = $props();

	const runId = $derived(openRun.run!.id);
	let timed = $state<Timed | null>(null);
	const phases = $derived(timeline(step, timed?.extra));
	const logged = $derived(setsOf(openRun.run!, step.id));

	// A skip or End set that lands on the end is the person ending the step,
	// not the timer running out.
	let cut = false;

	// The page keeps this step's last look while auto-next opens the next one.
	let advancing = $state(false);

	// A per-side set logs two rows, one for each side.
	const sides = $derived(step.per_side ? 2 : 1);
	let weight = $state<number | null>(null);
	let now = $state(Date.now());

	$effect.pre(() => {
		const stored = readTimers(runId).timed;
		const mine = stored?.step === step.id && !isFinished(step) ? stored : null;
		timed = mine;
		cut = false;
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

	// Each block logs a row once its hangs are over, even if the phone slept
	// through it.
	$effect(() => {
		if (!timed) return;
		const done = rows(phases, ms, timed.short);
		if (done.length > logged.length) log(done);
		if (pos.index >= phases.length) ranOut();
	});

	// When the last hang runs out on its own, a fresh timed step that comes
	// next starts with its own prep, so a routine runs with no touch. A timer
	// that ran out long ago, while the phone slept, only finishes.
	function ranOut() {
		const late = ms - startOf(phases, phases.length) > 2000;
		const run = openRun.run!;
		const after = nextStop(run, step.id)?.step;
		const steps = stepsOf(run);
		stop();
		if (cut || late || !after || after.kind !== 'timed_reps' || !canTime(after) || setsOf(run, after.id).length) return;
		if (steps.indexOf(after) < steps.indexOf(step)) return;
		advancing = true;
		writeTimers(runId, { ...readTimers(runId), timed: { step: after.id, clock: newClock(Date.now()), short: {}, weight } });
		void goto(nextHref).finally(() => (advancing = false));
	}

	// A tick in each of the last three seconds of a phase, and a high tone as
	// a hang starts. A key per sound plays each one once.
	let played = '';
	$effect(() => {
		if (!timed || !phase || timed.clock.pausedAt !== null) return;
		const secs = Math.ceil(pos.left / 1000);
		const key = `${pos.index}:${secs}`;
		if (key === played) return;
		const fresh = phase.ms - pos.left < 1000;
		if (fresh && phase.kind === 'hang') {
			tone(1046, 300);
			navigator.vibrate?.(300);
		} else if (fresh) {
			tone(523, 200);
			navigator.vibrate?.(150);
		} else if (secs <= 3) tone(784, 80);
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
		cut = pos.index + 1 >= phases.length;
		save({ ...timed!, clock: jump(timed!.clock, t, startOf(phases, pos.index + 1)) });
	}

	function addRest() {
		const extra = timed!.extra ?? {};
		save({ ...timed!, extra: { ...extra, [pos.index]: (extra[pos.index] ?? 0) + 30_000 } });
	}

	function cutSet() {
		const { clock, short } = endSet(phases, timed!.clock, Date.now(), timed!.short);
		cut = at(phases, elapsed(clock, Date.now())).index >= phases.length;
		save({ ...timed!, clock, short });
	}

	function endExercise() {
		const done = rows(phases, ms, timed!.short, true);
		if (done.length > logged.length) log(done);
		stop();
	}

	async function endSession() {
		endExercise();
		await goto(`/run/${runId}/finish`);
	}

	const words = { hang: 'Work', rest: 'Rest' };
	const icons = { prep: 'hourglass', hang: 'bolt', rest: 'wave' } as const;

	// The flip cards on each field.
	const cards = {
		run: { card: 'bg-[#0f2a1c]', digit: 'text-rest-on-hero', split: 'bg-run' },
		prep: { card: 'bg-prep-card', digit: 'text-prep-digit', split: 'bg-prep' },
		hang: { card: 'bg-on-hang', digit: 'text-hang-digit', split: 'bg-hang' },
		rest: { card: 'bg-rest-plate', digit: 'text-rest-glyph', split: 'bg-rest-field' }
	};

	const s = (secs: number | null) => `${secs ?? 0} s`;

	// What comes after this phase, under the NEXT label: the next phase of
	// this step, or the next step once its last hang is on.
	function then(p: Phase) {
		const rep = `Rep ${p.rep} · ${s(step.rep_seconds)}`;
		const side = p.side && p.side !== phase?.side ? `${p.side} side · ` : '';
		if (phase && p.set !== phase.set) return `Set ${p.set} · ${side}${rep}`;
		return side + rep;
	}

	const finished = $derived(isFinished(step) && !leaving && !advancing);
	const setsLogged = $derived(Math.floor(logged.length / sides));
	const paused = $derived(timed?.clock.pausedAt != null);
	const repsDone = $derived(phase ? (phase.kind === 'rest' ? phase.rep : phase.rep - 1) : 0);
	const setOver = $derived(phase?.kind === 'rest' && phase.rep === step.reps && upcoming?.set !== phase.set);
	const setsDone = $derived(phase ? (setOver ? phase.set : phase.set - 1) : 0);

	const status = $derived.by(() => {
		if (finished) return `Done · ${setsLogged} of ${step.sets}`;
		if (!timed || !phase) return setsLogged ? `Ready · ${setsLogged} of ${step.sets} logged` : 'Ready';
		// On a per-side step a hang names its side instead of "Work", so the
		// line stays one line.
		if (phase.kind === 'prep') return phase.side ? `Prep · ${phase.side}` : 'Prep';
		const word = phase.kind === 'hang' && phase.side ? phase.side : words[phase.kind];
		return `${word} · Rep ${phase.rep} of ${step.reps}`;
	});

	const nextLine = $derived.by(() => {
		if (finished) return next;
		if (!timed || !phase) {
			const work = setsLogged
				? `set ${setsLogged + 1}`
				: `${step.sets && step.sets > 1 ? `${step.sets} × ` : ''}${step.reps} × ${s(step.rep_seconds)}`;
			return step.prep_seconds ? `Prep ${s(step.prep_seconds)}, then ${work}` : work[0].toUpperCase() + work.slice(1);
		}
		if (!upcoming) return next;
		if (upcoming.kind === 'rest') return `Rest · ${s(Math.round(upcoming.ms / 1000))}`;
		return then(upcoming);
	});

	const secs = $derived(timed && phase ? Math.ceil(pos.left / 1000) : (step.rep_seconds ?? 0));
	const clock = $derived(secs >= 60 ? `${Math.floor(secs / 60)}${String(secs % 60).padStart(2, '0')}` : String(secs));
	const look = $derived(timed && phase ? cards[phase.kind] : cards.run);
	const ready = $derived(!timed && !finished);
</script>

{#snippet bar(label: string, at: number, of: number, done: number)}
	<div class="flex h-10 items-center gap-[18px]">
		<span class="min-w-[92px] shrink-0 text-xl font-semibold text-(--fg2)">
			{label} <b class="mx-px font-[family-name:var(--font-digits)] text-[34px] leading-none font-bold text-(--fg)">{at}</b>/{of}
		</span>
		<span class="flex flex-1 gap-1.5" aria-hidden="true">
			{#each { length: of }, i (i)}
				<i class="h-3 flex-1 rounded-full bg-current {i < done ? '' : i === done && timed ? 'opacity-55' : 'opacity-20'}"></i>
			{/each}
		</span>
	</div>
{/snippet}

<RunPage
	{step}
	look={timed && phase ? phase.kind : 'run'}
	icon={finished ? 'check' : timed && phase ? icons[phase.kind] : 'hourglass'}
	{status}
	next={nextLine}
	menu={timed
		? [
				{ label: 'End set', onclick: cutSet },
				{ label: 'End exercise', onclick: endExercise },
				{ label: 'Add set', onclick: () => openRun.addSet(step) },
				...menu.map((m) => (m.label === 'Finish session' ? { ...m, onclick: endSession } : m))
			]
		: [{ label: 'Add set', onclick: () => openRun.addSet(step) }, ...menu]}
	{error}
	band={timed && phase ? 1 - pos.left / phase.ms : null}
>
	{#if finished}
		<div class="flex flex-1 flex-col items-center justify-center gap-3.5 pb-10">
			<span class="flex size-[132px] items-center justify-center rounded-full bg-tint text-on-tint"><Icon name="check" size="64px" stroke={3} /></span>
			<span class="text-[17px] font-bold text-(--fg2)">{setsLogged} of {step.sets} {step.sets === 1 ? 'set' : 'sets'} logged</span>
		</div>
	{:else}
		<div
			class="mt-6 flex shrink-0 items-end justify-center gap-2 {ready
				? '[--h:min(196px,20svh,48vw)]'
				: clock.length === 1
					? '[--h:min(300px,34svh,76vw)]'
					: '[--h:min(236px,28svh,56vw)]'} {paused ? 'opacity-55' : ''}"
			role="img"
			aria-live="off"
			aria-label="{secs} {secs === 1 ? 'second' : 'seconds'}{timed ? ' left' : ''}"
		>
			{#each clock.split('') as digit, i (i)}
				{#if secs >= 60 && i === clock.length - 2}
					<div class="flex w-4 flex-col items-center gap-[calc(var(--h)*0.127)] self-center" aria-hidden="true">
						<i class="size-4 rounded-full bg-current {look.digit}"></i>
						<i class="size-4 rounded-full bg-current {look.digit}"></i>
					</div>
				{/if}
				<TimerFlap {digit} wide={clock.length === 1} {look} />
			{/each}
			{#if secs < 60}
				<span class="ml-0.5 pb-5 font-[family-name:var(--font-digits)] text-[44px] leading-none font-bold">s</span>
			{/if}
		</div>

		<div class="mt-4 flex flex-col gap-1.5 px-6">
			{@render bar('Set', phase?.set ?? Math.min(setsLogged + 1, step.sets ?? 1), step.sets ?? 1, timed ? setsDone : setsLogged)}
			{@render bar('Rep', timed && phase ? (phase.kind === 'rest' ? Math.min(phase.rep, step.reps ?? 1) : phase.rep) : 1, step.reps ?? 1, timed ? repsDone : 0)}
		</div>

		{#if ready}
			<div class="mx-4 mt-3">
				<Stepper label="Added weight, kg" step={2.5} min={-200} placeholder="–" bind:value={weight} />
			</div>
		{:else if weight}
			<p class="mt-2 px-6 text-[15px] font-semibold text-(--fg2)">{weight > 0 ? '+' : ''}{weight} kg</p>
		{/if}
	{/if}

	{#snippet buttons()}
		{#if finished}
			<a href={nextHref} class="run-btn bg-live text-on-live">
				{last ? 'Back to session' : 'Next exercise'}<span class="run-icon"><Icon name="arrow-right" size="16px" stroke={2.6} /></span>
			</a>
		{:else if !timed}
			<button type="button" class="run-btn bg-tint text-on-tint" onclick={start}>
				<span class="run-icon"><Icon name="play" size="16px" /></span>{logged.length ? 'Continue' : 'Start'}
			</button>
			<button type="button" class="run-chip" onclick={skip}><span class="run-icon"><Icon name="skip" size="16px" /></span>Skip</button>
		{:else}
			<button type="button" class="run-btn {paused && phase?.kind !== 'hang' ? 'bg-tint text-on-tint' : 'bg-(--fg) text-(--field)'}" onclick={pause}>
				<span class="run-icon"><Icon name={paused ? 'play' : 'pause'} size="16px" stroke={3} /></span>{paused ? 'Resume' : 'Pause'}
			</button>
			{#if phase?.kind === 'rest'}
				<button type="button" class="run-chip pl-[18px]" onclick={addRest}>+30 s</button>
			{/if}
			<button type="button" class="run-chip" onclick={skipPhase}><span class="run-icon"><Icon name="skip" size="16px" /></span>Skip</button>
		{/if}
	{/snippet}
</RunPage>
