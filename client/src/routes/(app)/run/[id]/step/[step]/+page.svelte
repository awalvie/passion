<script lang="ts">
	import { page } from '$app/state';
	import NavBar from '$lib/NavBar.svelte';
	import Notes from '$lib/Notes.svelte';
	import SaveStatus from '$lib/SaveStatus.svelte';
	import SetsPlayer from '$lib/SetsPlayer.svelte';
	import { stepsOf } from '$lib/run';
	import { openRun } from '$lib/runState.svelte';
	import { stepMeta } from '$lib/template';

	const run = $derived(openRun.run!);
	const step = $derived(openRun.step(page.params.step!));
	const section = $derived(run.sections.find((s) => s.items.some((i) => i.step?.id === step?.id)));
	const position = $derived(stepsOf(run).findIndex((s) => s.id === step?.id) + 1);
</script>

<svelte:head><title>{step?.name ?? run.name}</title></svelte:head>

<NavBar back={{ href: `/run/${run.id}`, label: 'Session' }} />

{#if step}
	<div class="flex flex-col gap-4 px-4 pt-2 pb-[calc(env(safe-area-inset-bottom)+2rem)]">
		<SaveStatus />

		<header class="flex flex-col gap-1">
			<p class="text-sm text-ink-2">{section?.name} · {position} of {stepsOf(run).length}</p>
			<h1 class="text-3xl font-bold">{step.name}</h1>
			<p class="text-base text-ink-2">{stepMeta(step)}</p>
		</header>

		{#if step.notes}
			<details class="rounded-2xl bg-surface shadow-sm">
				<summary class="flex cursor-pointer items-center gap-3 px-4 py-3 text-base">
					<span class="font-semibold">How to</span>
					<span class="min-w-0 flex-1 truncate text-ink-2">{step.notes.split('\n')[0]}</span>
				</summary>
				<Notes text={step.notes} class="px-4 pb-4 text-base text-ink-2" />
			</details>
		{/if}

		{#if step.kind === 'reps_and_sets'}
			<SetsPlayer {step} />
		{:else}
			<p class="text-base text-ink-2">This kind of exercise cannot be logged here yet.</p>
		{/if}
	</div>
{/if}
