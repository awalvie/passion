<script lang="ts">
	import { goto } from '$app/navigation';
	import Icon from './Icon.svelte';
	import { tone, unlock } from './audio';
	import Menu from './Menu.svelte';
	import Notes from './Notes.svelte';
	import { isFinished, secondsSince, setsOf, type RunStep } from './run';
	import { openRun } from './runState.svelte';
	import SaveStatus from './SaveStatus.svelte';
	import { plainText } from './text';
	import { elapsed, newClock, togglePause } from './timeline';
	import { formatClock, readTimers, sessionClock, writeTimers, type Timed } from './timerStore';

	let { step, nextHref, last }: { step: RunStep; nextHref: string; last: boolean } = $props();

	const run = $derived(openRun.run!);
	const runId = $derived(run.id);
	const logged = $derived(setsOf(run, step.id));
	const target = $derived((step.duration_seconds ?? 0) * 1000);
	const section = $derived(run.sections.find((s) => s.items.some((i) => i.step?.id === step.id)));
	const siblings = $derived(section?.items.flatMap((i) => (i.step ? [i.step] : [])) ?? []);
	const video = $derived(step.media?.find((m) => m.url));
	const howTo = $derived(plainText(step.notes ?? '').trim());

	let timed = $state<Timed | null>(null);
	let now = $state(Date.now());

	// Each is the id of the step it was set on, so the next step starts clear.
	let leaving = $state<string | null>(null);
	let noting = $state<string | null>(null);

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

	// The cover stays up until the next step loads, so the page under it does
	// not flash its finished state.
	async function next() {
		leaving = step.id;
		if (timed && ms >= 1000) done(ms);
		else {
			if (timed) save(null);
			openRun.finish(step);
		}
		await goto(nextHref);
		leaving = null;
	}

	async function skip() {
		leaving = step.id;
		if (timed) save(null);
		openRun.skip(step);
		await goto(nextHref);
		leaving = null;
	}

	function saveNote(text: string) {
		step.run_notes = text.trim() || null;
		openRun.saveBody();
	}

	const clock = $derived(
		!timed
			? formatClock(target / 1000)
			: target
				? formatClock((target - ms) / 1000)
				: formatClock(Math.floor(ms / 1000))
	);
	const clockSize = $derived(clock.length <= 4 ? 'text-[min(168px,43vw)]' : clock.length === 5 ? 'text-[min(136px,35vw)]' : 'text-[min(108px,27vw)]');
	const glass = 'bg-on-hero/10';
</script>

{#if !isFinished(step) || leaving === step.id}
	<section class="fixed inset-0 z-40 overflow-hidden bg-hero text-on-hero" aria-label="Open exercise">
		<div
			class="relative mx-auto flex h-full w-full max-w-[430px] flex-col pt-[env(safe-area-inset-top)] pb-[calc(env(safe-area-inset-bottom)+1rem)]"
		>
			<div class="relative flex h-14 shrink-0 items-center justify-between px-4">
				<a href="/run/{runId}" class="inline-flex h-11 items-center gap-0.5 rounded-full pr-4 pl-2.5 text-[15px] font-bold {glass}">
					<Icon name="chevron-left" size="20px" stroke={2.4} />
					Session
				</a>
				<span
					class="absolute top-1/2 left-1/2 flex h-11 -translate-x-1/2 -translate-y-1/2 items-center gap-2 rounded-full px-4 font-[family-name:var(--font-digits)] text-xl font-bold shadow-[inset_0_0_0_1px_var(--live)] {glass}"
					aria-label="Session time"
				>
					<i class="size-2 rounded-full bg-live shadow-[0_0_0_4px_var(--live-halo)]"></i>
					{sessionClock(secondsSince(run.started_at, now))}
				</span>
				<Menu look={glass} items={[{ label: 'Finish session', onclick: () => goto(`/run/${runId}/finish`) }]} />
			</div>
			<div class="flex flex-col gap-2 px-4 empty:hidden"><SaveStatus /></div>

			<div class="px-6 pt-5 text-center">
				{#if section}
					<p class="text-xs font-bold tracking-[0.06em] text-tint uppercase">
						{section.name} · {siblings.findIndex((s) => s.id === step.id) + 1} of {siblings.length}
					</p>
				{/if}
				<h1 class="mt-1.5 line-clamp-2 text-[40px] leading-[1.1] font-bold tracking-[-0.02em] break-words">{step.name}</h1>
			</div>

			<p
				class="mt-8 text-center font-[family-name:var(--font-digits)] leading-none font-bold tracking-[-0.02em] whitespace-nowrap {clockSize}"
				role="timer"
				aria-live="off"
			>
				{clock}
			</p>

			{#if paused}
				<p class="mt-2 px-6 text-center text-[15px] font-semibold text-on-hero-2">Paused</p>
			{/if}
			{#if howTo}
				<div class="mt-3 min-h-0 flex-1 overflow-y-auto overscroll-contain px-6 text-[15px] font-semibold text-on-hero-2">
					<Notes text={howTo} />
				</div>
			{/if}

			{#if video}
				<a
					href={video.url}
					target="_blank"
					rel="noopener noreferrer"
					class="mt-3 flex h-11 items-center gap-2 self-center rounded-full bg-on-hero/10 px-4 text-[15px] font-bold"
				>
					<Icon name="play" size="1rem" />Watch how
				</a>
			{/if}

			<div class="mt-auto flex flex-col gap-3 px-4 pt-4" onpointerdown={unlock} role="group" aria-label="Exercise">
				{#if noting === step.id || step.run_notes}
					<textarea
						class="min-h-24 rounded-3xl p-4 text-[15px] font-semibold text-on-hero outline-none placeholder:text-on-hero-2 {glass}"
						placeholder="How did it go?"
						aria-label="Note"
						value={step.run_notes ?? ''}
						oninput={(e) => (step.run_notes = e.currentTarget.value)}
						onchange={(e) => saveNote(e.currentTarget.value)}
					></textarea>
				{/if}

				{#if !timed}
					<button type="button" class="flex h-30 items-center justify-center gap-3 rounded-[36px] bg-tint text-[28px] font-bold text-on-tint shadow-tint" onclick={start}>
						<Icon name="play" size="30px" />
						Start
					</button>
				{:else}
					<button type="button" class="flex h-30 items-center justify-center gap-3 rounded-[36px] text-[28px] font-bold {glass}" onclick={pause}>
						{#if paused}
							<Icon name="play" size="30px" />
							Resume
						{:else}
							<svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round" aria-hidden="true"><path d="M8 5v14M16 5v14" /></svg>
							Pause
						{/if}
					</button>
				{/if}

				<div class="flex gap-2">
					<button
						type="button"
						class="flex h-15 flex-1 items-center justify-center gap-2 rounded-full text-base font-bold {glass}"
						onclick={() => (noting = noting === step.id ? null : step.id)}
					>
						<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M7 3.5h7l4 4v13H7z" /><path d="M14 3.5v4h4M10 12h5M10 16h5" /></svg>
						Note
					</button>
					<button type="button" class="flex h-15 flex-1 items-center justify-center gap-2 rounded-full text-base font-bold {glass}" onclick={skip}>
						<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M6 5.5v13l9-6.5z" fill="currentColor" /><path d="M18.5 5v14" /></svg>
						Skip
					</button>
					<button type="button" class="flex h-15 flex-1 items-center justify-center gap-2 rounded-full text-base font-bold {glass}" onclick={next}>
						{last ? 'Done' : 'Next'}
						<Icon name="chevron-right" size="18px" stroke={2.2} />
					</button>
				</div>
			</div>
		</div>
	</section>
{:else if logged[0]?.seconds}
	<p class="flex h-11 items-center gap-2.5 rounded-3xl bg-surface px-4 text-[15px] font-bold shadow-card">
		<span class="flex size-6 shrink-0 items-center justify-center rounded-full bg-ink text-ground dark:bg-ink-2">
			<Icon name="check" size="0.75rem" stroke={3.2} />
		</span>
		Took {formatClock(logged[0].seconds)}
	</p>
{/if}
