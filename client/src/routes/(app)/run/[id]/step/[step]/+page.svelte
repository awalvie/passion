<script lang="ts">
	import Icon from '$lib/Icon.svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { describe } from '$lib/api';
	import Button from '$lib/Button.svelte';
	import ClimbsPlayer from '$lib/ClimbsPlayer.svelte';
	import FormError from '$lib/FormError.svelte';
	import Menu from '$lib/Menu.svelte';
	import NavBar from '$lib/NavBar.svelte';
	import Notes from '$lib/Notes.svelte';
	import OpenPlayer from '$lib/OpenPlayer.svelte';
	import SaveStatus from '$lib/SaveStatus.svelte';
	import SetsPlayer from '$lib/SetsPlayer.svelte';
	import { canTime } from '$lib/timeline';
	import TimerPlayer from '$lib/TimerPlayer.svelte';
	import { keepAwake } from '$lib/wakeLock';
	import { climbsOf, isFinished, nextStop, secondsSince, setsOf, stepsOf, stopHref } from '$lib/run';
	import { openRun } from '$lib/runState.svelte';
	import { choiceMeta, stepMeta } from '$lib/template';
	import { plainText } from '$lib/text';

	const run = $derived(openRun.run!);
	const step = $derived(openRun.step(page.params.step!));
	const section = $derived(run.sections.find((s) => s.items.some((i) => i.step?.id === step?.id)));
	const position = $derived(stepsOf(run).findIndex((s) => s.id === step?.id) + 1);
	const next = $derived(step ? nextStop(run, step.id) : undefined);
	const nextHref = $derived(stopHref(run.id, next));
	const logged = $derived(step ? setsOf(run, step.id).length + climbsOf(run, step.id).length : 0);

	$effect(() => {
		if (step) openRun.opened(step.id);
	});

	// The phone lies on the floor during a set, with chalky hands.
	$effect(() => keepAwake());

	const howTo = $derived(plainText(step?.notes ?? ''));
	const media = $derived(step?.media?.find((m) => m.thumb_url && m.url));

	let now = $state(Date.now());
	$effect(() => {
		const tick = setInterval(() => (now = Date.now()), 30_000);
		return () => clearInterval(tick);
	});

	let noting = $state(false);

	const total = $derived(stepsOf(run).length);
	const sets = $derived(step?.kind === 'reps_and_sets' || step?.kind === 'timed_reps');

	function saveNote(text: string) {
		step!.run_notes = text.trim() || null;
		openRun.saveBody();
	}

	let error = $state('');

	async function discard() {
		error = '';
		try {
			await openRun.discard();
		} catch (e) {
			error = describe(e, (f) => f);
		}
	}

	// Skip keeps what is logged and ends the step, or skips a step with nothing.
	async function skip() {
		if (!step) return;
		if (logged) openRun.finish(step);
		else openRun.skip(step);
		await goto(nextHref);
	}
</script>

<svelte:head><title>{step?.name ?? run.name}</title></svelte:head>

{#snippet ctrl(label: string, paths: string[], onclick: () => void, filled = false)}
	<button type="button" class="flex w-20 flex-col items-center gap-1.5" {onclick}>
		<span class="flex size-12 items-center justify-center rounded-full bg-surface text-ink shadow-card">
			<svg viewBox="0 0 24 24" class="size-[22px]" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
				{#each paths as d, i (i)}<path {d} fill={filled && i === 0 ? 'currentColor' : 'none'} />{/each}
			</svg>
		</span>
		<span class="text-center text-xs leading-tight font-bold text-ink">{label}</span>
	</button>
{/snippet}

<NavBar back={{ href: `/run/${run.id}`, label: 'Session' }} title={step ? `${position} of ${total}` : ''} heading={false}>
	{#snippet actions()}
		<Menu
			items={[
				{ label: 'Finish session', onclick: () => goto(`/run/${run.id}/finish`) },
				{ label: 'Discard session', danger: true, onclick: discard }
			]}
		/>
	{/snippet}
</NavBar>

{#if step}
	<div class="flex flex-col gap-2.5 px-4 pt-0.5 pb-[calc(env(safe-area-inset-bottom)+2rem)]">
		<SaveStatus />
		<FormError message={error} />

		<header class="px-1 pb-0.5">
			{#if section}<p class="text-xs font-semibold tracking-[0.06em] text-ink-2 uppercase">{section.name}</p>{/if}
			<h1 class="mt-0.5 text-[32px] leading-[1.1] font-extrabold tracking-[-0.02em]">{step.name}</h1>
			<p class="mt-1 text-[15px] font-semibold text-ink-2">
				{#if step.kind === 'climbing'}
					{logged} {logged === 1 ? 'climb' : 'climbs'} logged · {Math.floor(secondsSince(run.started_at, now) / 60)} min in
				{:else}
					{stepMeta(step)}
				{/if}
			</p>
		</header>

		{#if step.notes}
			<details class="group rounded-3xl bg-surface shadow-card">
				<summary class="flex min-h-[68px] cursor-pointer list-none items-center gap-3.5 py-2 pr-3 {media ? 'pl-2' : 'pl-4'} [&::-webkit-details-marker]:hidden">
					{#if media}
						<a
							href={media.url}
							target="_blank"
							rel="noopener noreferrer"
							class="relative flex size-[52px] shrink-0 items-center justify-center overflow-hidden rounded-[14px] bg-well"
							aria-label="Play video"
						>
							<img src={media.thumb_url} alt="" class="absolute inset-0 size-full object-cover" />
							<span class="relative flex size-7 items-center justify-center rounded-full bg-white/90 pl-0.5 text-on-tint">
								<Icon name="play" size="0.875rem" />
							</span>
						</a>
					{/if}
					<span class="min-w-0 flex-1">
						<span class="mb-0.5 block text-xs font-semibold text-ink-2">How to</span>
						<span class="line-clamp-2 text-[15px] leading-[1.35] font-semibold text-ink group-open:hidden">{howTo.replace(/\s+/g, ' ')}</span>
					</span>
					<span class="flex shrink-0 text-ink-3 transition-transform group-open:rotate-90"><Icon name="chevron-right" size="18px" stroke={2.2} /></span>
				</summary>
				<Notes text={howTo} class="px-4 pb-4 text-[15px] leading-[1.35] font-semibold text-ink" />
			</details>
		{/if}

		{#if step.kind === 'timed_reps' && canTime(step)}
			<TimerPlayer {step} />
		{:else if step.kind === 'reps_and_sets' || step.kind === 'timed_reps'}
			<SetsPlayer {step} />
		{:else if step.kind === 'open'}
			<OpenPlayer {step} {nextHref} last={!next} />
		{:else if step.kind === 'climbing'}
			<ClimbsPlayer {step} />
		{:else}
			<p class="text-[15px] text-ink-2">This kind of exercise cannot be logged here yet.</p>
		{/if}

		{#if isFinished(step)}
			<Button variant="live" href={nextHref}>{next ? 'Next exercise' : 'Back to session'}</Button>
		{/if}

		<div class="flex justify-center gap-7 pt-1.5">
			{#if sets && step.status !== 'skipped'}
				{@render ctrl('Add set', ['M12 5v14M5 12h14'], () => openRun.addSet(step!))}
			{/if}
			{@render ctrl('Note', ['M7 3.5h7l4 4v13H7z', 'M14 3.5v4h4M10 12h5M10 16h5'], () => (noting = !noting))}
			{#if step.status === 'skipped'}
				{@render ctrl('Undo skip', ['M9 14 4 9l5-5', 'M4 9h10.5a5.5 5.5 0 0 1 0 11H11'], () => openRun.unskip(step!))}
			{:else if !isFinished(step)}
				{#if logged && step.kind === 'climbing'}
					{@render ctrl('Done climbing', ['M5 12.5l4.5 4.5L19 7'], skip)}
				{:else}
					{@render ctrl(logged ? 'Skip the sets left' : 'Skip exercise', ['M6 5.5v13l9-6.5z', 'M18.5 5v14'], skip, true)}
				{/if}
			{/if}
		</div>

		{#if noting || step.run_notes}
			<textarea
				class="min-h-24 rounded-3xl bg-surface p-4 text-[15px] font-semibold text-ink shadow-card outline-none placeholder:text-ink-3"
				placeholder="How did it go?"
				aria-label="Note"
				value={step.run_notes ?? ''}
				onchange={(e) => saveNote(e.currentTarget.value)}
			></textarea>
		{/if}

		{#if next}
			<a href={nextHref} class="mt-1.5 flex items-center gap-3.5 rounded-3xl bg-surface py-2.5 pr-3.5 pl-4 shadow-card">
				<span class="w-[52px] shrink-0 text-xs leading-tight font-semibold tracking-[0.06em] text-ink-2 uppercase">Up next</span>
				<span class="min-w-0 flex-1">
					{#if next.step}
						<span class="block truncate text-[15px] font-bold">{next.step.name}</span>
						<span class="block truncate text-xs font-semibold text-ink-2">{stepMeta(next.step)}</span>
					{:else}
						<span class="block truncate text-[15px] font-bold">Pick {next.choice.name}</span>
						<span class="block truncate text-xs font-semibold text-ink-2">{choiceMeta(next.choice)}</span>
					{/if}
				</span>
				<span class="flex shrink-0 text-ink-3"><Icon name="chevron-right" size="18px" stroke={2.2} /></span>
			</a>
		{/if}
	</div>
{/if}
