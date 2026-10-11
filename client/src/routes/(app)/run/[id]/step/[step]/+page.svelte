<script lang="ts">
	import Icon from '$lib/Icon.svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { describe } from '$lib/api';
	import ClimbsPlayer from '$lib/ClimbsPlayer.svelte';
	import type { MenuItem } from '$lib/Menu.svelte';
	import NavBar from '$lib/NavBar.svelte';
	import OpenPlayer from '$lib/OpenPlayer.svelte';
	import RunPage from '$lib/RunPage.svelte';
	import SetsPlayer from '$lib/SetsPlayer.svelte';
	import { canTime } from '$lib/timeline';
	import TimerPlayer from '$lib/TimerPlayer.svelte';
	import { keepAwake } from '$lib/wakeLock';
	import { climbsOf, nextStop, setsOf, stopHref } from '$lib/run';
	import { openRun } from '$lib/runState.svelte';
	import { stopLine } from '$lib/template';

	const run = $derived(openRun.run!);
	const step = $derived(openRun.step(page.params.step!));
	const next = $derived(step ? nextStop(run, step.id) : undefined);
	const nextHref = $derived(stopHref(run.id, next));
	const logged = $derived(step ? setsOf(run, step.id).length + climbsOf(run, step.id).length : 0);

	$effect(() => {
		if (step) openRun.opened(step.id);
	});

	// The phone lies on the floor during a set, with chalky hands.
	$effect(() => keepAwake());

	let error = $state('');

	async function discard() {
		error = '';
		try {
			await openRun.discard();
		} catch (e) {
			error = describe(e, (f) => f);
		}
	}

	// The step being left, so its Next button does not flash before the next
	// step loads.
	let leaving = $state<string | null>(null);

	// leave ends the step and goes on. The page keeps the step's look until
	// the next one loads.
	async function leave(end: () => void) {
		leaving = step!.id;
		end();
		await goto(nextHref);
		leaving = null;
	}

	// Skip keeps what is logged and ends the step, or skips a step with nothing.
	function skip() {
		void leave(() => (logged ? openRun.finish(step!) : openRun.skip(step!)));
	}

	const menu: MenuItem[] = [
		{ label: 'Finish session', onclick: () => goto(`/run/${run.id}/finish`) },
		{ label: 'Discard session', danger: true, onclick: discard }
	];
</script>

<svelte:head><title>{step?.name ?? run.name}</title></svelte:head>

{#if step && step.status === 'skipped' && leaving !== step.id}
	<RunPage {step} next={stopLine(next)} {menu} {error}>
		<p class="mt-10 px-5 text-[17px] font-semibold text-(--fg2)">You skipped this exercise. Undo brings it back with nothing logged.</p>
		{#snippet buttons()}
			<button type="button" class="run-btn bg-(--glass)" onclick={() => openRun.unskip(step!)}>
				<span class="run-icon"><Icon name="undo-2" size="16px" stroke={2.6} /></span>Undo skip
			</button>
			<a href={nextHref} class="run-btn bg-live text-on-live">
				{next ? 'Next' : 'Session'}<span class="run-icon"><Icon name="arrow-right" size="16px" stroke={2.6} /></span>
			</a>
		{/snippet}
	</RunPage>
{:else if step?.kind === 'timed_reps' && canTime(step)}
	<TimerPlayer {step} next={stopLine(next)} {nextHref} last={!next} {menu} {error} leaving={leaving === step.id} {skip} />
{:else if step && (step.kind === 'reps_and_sets' || (step.kind === 'timed_reps' && !canTime(step)))}
	<SetsPlayer
		{step}
		next={stopLine(next)}
		{nextHref}
		last={!next}
		{menu}
		{error}
		leaving={leaving === step.id}
		{leave}
		{skip}
	/>
{:else if step?.kind === 'open'}
	<OpenPlayer {step} next={stopLine(next)} {nextHref} last={!next} {menu} {error} leaving={leaving === step.id} {leave} />
{:else if step?.kind === 'climbing'}
	<ClimbsPlayer {step} next={stopLine(next)} {nextHref} last={!next} {menu} {error} leaving={leaving === step.id} {leave} {skip} />
{:else}
	<NavBar back={{ href: `/run/${run.id}`, label: 'Session' }} title="" heading={false} />
{/if}
