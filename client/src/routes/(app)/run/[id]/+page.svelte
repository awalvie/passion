<script lang="ts">
	import Button from '$lib/Button.svelte';
	import Icon from '$lib/Icon.svelte';
	import NavBar from '$lib/NavBar.svelte';
	import { currentStep, isFinished, secondsSince, stepsOf, type RunStep } from '$lib/run';
	import { openRun } from '$lib/runState.svelte';
	import { choiceMeta, stepMeta } from '$lib/template';

	const run = $derived(openRun.run!);
	const steps = $derived(stepsOf(run));
	const finished = $derived(steps.filter(isFinished).length);
	const current = $derived(currentStep(run));
	const section = $derived(
		current ? run.sections.findIndex((s) => s.items.some((i) => i.step?.id === current.id)) : -1
	);

	let now = $state(Date.now());
	$effect(() => {
		const tick = setInterval(() => (now = Date.now()), 1000);
		return () => clearInterval(tick);
	});
	const minutes = $derived(
		Math.floor((run.finished_at ? (run.elapsed_seconds ?? 0) : secondsSince(run.started_at, now)) / 60)
	);

	function mark(s: RunStep): 'done' | 'skipped' | 'now' | 'todo' {
		if (s.status === 'skipped') return 'skipped';
		if (isFinished(s)) return 'done';
		return s.id === current?.id ? 'now' : 'todo';
	}
</script>

<svelte:head><title>{run.name}</title></svelte:head>

<NavBar title={run.name} back={{ href: '/', label: 'Today' }} />

<div class="flex flex-col gap-6 px-4 pt-2 pb-[calc(env(safe-area-inset-bottom)+6rem)]">
	<section class="rounded-2xl bg-surface p-4 shadow-sm">
		<div class="flex items-baseline justify-between gap-3">
			<p class="text-3xl font-semibold tabular-nums">{minutes} min</p>
			<p class="text-base text-ink-2">
				{#if run.finished_at}
					Finished
				{:else if section >= 0}
					Section {section + 1} of {run.sections.length}
				{:else}
					All done
				{/if}
			</p>
		</div>
		<div
			class="mt-3 h-2 overflow-hidden rounded-full bg-line"
			role="progressbar"
			aria-valuemin={0}
			aria-valuemax={steps.length}
			aria-valuenow={finished}
		>
			<div class="h-full rounded-full bg-live" style="width: {steps.length ? (finished / steps.length) * 100 : 0}%"></div>
		</div>
		<p class="mt-2 text-sm text-ink-2">{finished} of {steps.length} exercises</p>
	</section>

	{#each run.sections as s, i (i)}
		<section class="flex flex-col gap-2">
			<h2 class="px-1 text-sm font-semibold text-ink-2">{s.name}</h2>
			<ul class="overflow-hidden rounded-2xl bg-surface shadow-sm">
				{#each s.items as item, j (item.step?.id ?? item.choice?.id ?? j)}
					<li class="border-line [&:not(:first-child)]:border-t">
						{#if item.step}
							{@const m = mark(item.step)}
							<a href="/run/{run.id}/step/{item.step.id}" class="flex items-center gap-3 px-4 py-3">
								<span
									class="flex size-6 shrink-0 items-center justify-center rounded-full {m === 'done'
										? 'bg-tint text-on-tint'
										: m === 'now'
											? 'bg-live'
											: 'border-2 border-line'}"
									aria-label={m}
								>
									{#if m === 'done'}<Icon name="check" size="0.875rem" />{/if}
								</span>
								<span class="min-w-0 flex-1">
									<span class="block truncate text-base {m === 'skipped' ? 'text-ink-2 line-through' : ''}">
										{item.step.name}
									</span>
									<span class="block truncate text-sm text-ink-2">{stepMeta(item.step)}</span>
								</span>
								<Icon name="chevron-right" size="1rem" />
							</a>
						{:else}
							<div class="flex items-center gap-3 px-4 py-3 text-ink-2">
								<span class="size-6 shrink-0 rounded-full border-2 border-dashed border-line"></span>
								<span class="min-w-0 flex-1">
									<span class="block truncate text-base">{item.choice.name}</span>
									<span class="block truncate text-sm">{choiceMeta(item.choice)} · not picked</span>
								</span>
							</div>
						{/if}
					</li>
				{:else}
					<li class="px-4 py-3 text-sm text-ink-2">No exercises</li>
				{/each}
			</ul>
		</section>
	{/each}
</div>

{#if current && !run.finished_at}
	<div class="fixed inset-x-0 bottom-0 z-20 mx-auto w-full max-w-[430px] bg-ground/90 px-4 pt-3 pb-[calc(env(safe-area-inset-bottom)+0.75rem)] backdrop-blur-md">
		<Button variant="live" href="/run/{run.id}/step/{current.id}">Continue</Button>
	</div>
{/if}
