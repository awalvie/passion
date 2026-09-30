<script lang="ts">
	import { goto } from '$app/navigation';
	import { describe, request } from '$lib/api';
	import Button from '$lib/Button.svelte';
	import FormError from '$lib/FormError.svelte';
	import Icon from '$lib/Icon.svelte';
	import Menu from '$lib/Menu.svelte';
	import NavBar from '$lib/NavBar.svelte';
	import { currentStep, isFinished, secondsSince, stepsOf, type RunStep } from '$lib/run';
	import { openRun } from '$lib/runState.svelte';
	import { choiceMeta, stepMeta, type Step } from '$lib/template';
	import type { Exercise } from '$lib/exercise';
	import ExercisePicker from '$lib/ExercisePicker.svelte';

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

	let error = $state('');

	let choosing = $state<string | null>(null);
	let picked = $state<number[]>([]);

	function openChoice(id: string) {
		choosing = choosing === id ? null : id;
		picked = [];
	}

	let library = $state<Exercise[] | null>(null);

	async function openLibrary() {
		error = '';
		try {
			const { exercises } = await request<{ exercises: Exercise[] }>('GET', '/api/v1/exercises');
			library = exercises.filter((e) => !e.retired_at);
		} catch (e) {
			error = describe(e, (f) => f);
		}
	}

	function pick(id: string, options: Step[]) {
		openRun.pick(id, options);
		choosing = null;
	}

	async function discard() {
		if (!confirm(`Discard “${run.name}”? Everything logged in it is deleted.`)) return;
		error = '';
		try {
			await request('DELETE', `/api/v1/runs/${run.id}`);
			await goto('/');
			openRun.forget();
		} catch (e) {
			error = describe(e, (f) => f);
		}
	}

	function mark(s: RunStep): 'done' | 'skipped' | 'now' | 'todo' {
		if (s.status === 'skipped') return 'skipped';
		if (isFinished(s)) return 'done';
		return s.id === current?.id ? 'now' : 'todo';
	}
</script>

<svelte:head><title>{run.name}</title></svelte:head>

<NavBar title={run.name} back={{ href: '/', label: 'Today' }}>
	{#snippet actions()}
		<Menu
			items={[
				{ label: 'Finish session', onclick: () => goto(`/run/${run.id}/finish`) },
				{ label: 'Discard session', danger: true, onclick: discard }
			]}
		/>
	{/snippet}
</NavBar>

<div class="px-4">
	<FormError message={error} />
</div>

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
							{@const c = item.choice}
							<button
								type="button"
								class="flex w-full items-center gap-3 px-4 py-3 text-left"
								aria-expanded={choosing === c.id}
								onclick={() => openChoice(c.id)}
							>
								<span class="size-6 shrink-0 rounded-full border-2 border-dashed border-line"></span>
								<span class="min-w-0 flex-1">
									<span class="block truncate text-base">{c.name}</span>
									<span class="block truncate text-sm text-ink-2">{choiceMeta(c)} · tap to pick</span>
								</span>
							</button>
							{#if choosing === c.id}
								<div class="flex flex-col gap-1 px-4 pb-4">
									{#each c.options as o, k (k)}
										<label class="flex items-center gap-3 rounded-xl px-2 py-2">
											<input type="checkbox" class="size-5 accent-tint" bind:group={picked} value={k} />
											<span class="min-w-0 flex-1">
												<span class="block text-base">{o.name}</span>
												<span class="block text-sm text-ink-2">{stepMeta(o)}</span>
											</span>
										</label>
									{/each}
									<Button
										disabled={picked.length < Math.max(1, c.pick)}
										onclick={() => pick(c.id, picked.map((k) => c.options[k]))}
									>
										Use {picked.length === 1 ? 'this' : 'these'}
									</Button>
								</div>
							{/if}
						{/if}
					</li>
				{:else}
					<li class="px-4 py-3 text-sm text-ink-2">No exercises</li>
				{/each}
			</ul>
		</section>
	{/each}

	{#if library}
		<div class="rounded-2xl bg-surface p-4 shadow-sm">
			<ExercisePicker
				id="add-exercise"
				exercises={library}
				pick={(e) => {
					openRun.addStep(e);
					library = null;
				}}
			/>
		</div>
	{:else}
		<Button variant="secondary" onclick={openLibrary}>Add exercise</Button>
	{/if}
</div>

{#if !run.finished_at && (current || steps.length)}
	<div class="fixed inset-x-0 bottom-0 z-20 mx-auto w-full max-w-[430px] bg-ground/90 px-4 pt-3 pb-[calc(env(safe-area-inset-bottom)+0.75rem)] backdrop-blur-md">
		{#if current}
			<Button variant="live" href="/run/{run.id}/step/{current.id}">Continue</Button>
		{:else}
			<Button href="/run/{run.id}/finish">Finish session</Button>
		{/if}
	</div>
{/if}
