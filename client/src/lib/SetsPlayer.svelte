<script lang="ts">
	import { untrack } from 'svelte';
	import { request } from './api';
	import Icon from './Icon.svelte';
	import type { MenuItem } from './Menu.svelte';
	import RestCard from './RestCard.svelte';
	import { isFinished, setsOf, type HistorySession, type RunStep, type SetFields } from './run';
	import { openRun } from './runState.svelte';
	import RunPage from './RunPage.svelte';
	import type { NextLine } from './template';
	import Stepper from './Stepper.svelte';

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
		next: NextLine;
		nextHref: string;
		last: boolean;
		menu: MenuItem[];
		error: string;
		leaving: boolean;
		skip: () => void;
	} = $props();

	// A per-side set logs two rows, left then right. The server has no side
	// column, so the order carries it.
	const sides = $derived(step.per_side ? 2 : 1);
	const logged = $derived(setsOf(openRun.run!, step.id));
	const planned = $derived((step.sets ?? 0) * sides);
	const row = $derived(logged.length + 1);
	const number = $derived(Math.ceil(row / sides));
	const side = (n: number) => (n % 2 ? 'Left' : 'Right');

	let reps = $state<number | null>(null);
	let weight = $state<number | null>(null);
	let lastTime = $state<HistorySession | null>(null);
	// The page stays mounted from one exercise to the next, so the history of
	// the one before can still be loaded when the steppers fill.
	const history = $derived(lastTime?.sets[0]?.exercise === step.exercise ? lastTime : null);

	// The newest finished run that logged this exercise. Without a signal the
	// sets start from the plan.
	$effect(() => {
		const exercise = step.exercise;
		let current = true;
		lastTime = null;
		request<{ sessions: HistorySession[] }>('GET', `/api/v1/exercises/${exercise}/history`)
			.then((h) => {
				if (!current) return;
				// A late answer fills in only what the person has not changed yet.
				const untouched = untrack(() => {
					const [r, w] = prefill();
					return reps === r && weight === w;
				});
				lastTime = h.sessions.find((s) => s.sets.length) ?? null;
				if (untouched) [reps, weight] = untrack(prefill);
			})
			.catch(() => {});
		return () => (current = false);
	});

	// Each set starts from the one before it, the first from the same set last
	// time, and else from the plan.
	function prefill(): [number | null, number | null] {
		const before = logged.at(-1);
		const then = history?.sets[logged.length] ?? history?.sets.at(-1);
		return [before?.reps ?? then?.reps ?? step.reps, before?.weight_kg ?? then?.weight_kg ?? null];
	}

	$effect.pre(() => {
		void step.id;
		void logged.length;
		[reps, weight] = untrack(prefill);
	});

	let rest = $state<ReturnType<typeof RestCard>>();

	function log() {
		const rows = logged.length + 1;
		openRun.setSets(step, [...logged, { reps, seconds: null, weight_kg: weight }]);
		if (planned && rows >= planned) openRun.finish(step);
		else if (rows % sides === 0) rest?.start();
	}

	const done = $derived(Array.from({ length: Math.ceil(logged.length / sides) }, (_, i) => logged.slice(i * sides, (i + 1) * sides)));
	let editing = $state<number | null>(null);
	let draft = $state<SetFields[]>([]);

	$effect.pre(() => {
		void step.id;
		editing = null;
	});

	function edit(i: number) {
		if (editing === i) return (editing = null);
		editing = i;
		draft = done[i].map((s) => ({ reps: s.reps, seconds: s.seconds, weight_kg: s.weight_kg }));
	}

	function saveEdit() {
		const at = editing! * sides;
		openRun.setSets(step, logged.map((s, k) => (k >= at && k < at + draft.length ? draft[k - at] : s)));
		[reps, weight] = prefill();
		editing = null;
	}

	function short(s: SetFields) {
		if (s.reps !== null && s.weight_kg !== null) return `${s.reps} × ${s.weight_kg} kg`;
		if (s.reps !== null) return `${s.reps} reps`;
		if (s.weight_kg !== null) return `${s.weight_kg} kg`;
		return 'Done';
	}

	const finished = $derived(isFinished(step) && !leaving);
	const setsDone = $derived(Math.floor(logged.length / sides));
	// The sets still to do after the one being logged now.
	const later = $derived(finished ? [] : Array.from({ length: Math.max(0, (step.sets ?? 0) - number) }, (_, i) => number + 1 + i));
	const plan = $derived(step.sets && step.reps ? `${step.sets} × ${step.reps}` : step.sets ? `${step.sets} sets` : step.reps ? `${step.reps} reps` : '');
	const lastWeight = $derived(history?.sets.find((s) => s.weight_kg !== null)?.weight_kg ?? null);
	const status = $derived(
		finished ? `Done · ${setsDone}${step.sets ? ` of ${step.sets}` : ''}` : `Set ${number}${step.sets ? ` of ${step.sets}` : ''}${sides === 2 ? ` · ${side(row)}` : ''}`
	);
</script>

{#snippet tick()}
	<span class="flex size-8 shrink-0 items-center justify-center rounded-full bg-tint text-on-tint">
		<Icon name="check" size="15px" stroke={3} />
	</span>
{/snippet}

{#snippet ring()}
	<i class="size-8 shrink-0 rounded-full border-2 border-(--fg2) opacity-70"></i>
{/snippet}

<RunPage {step} icon={finished ? 'check' : 'barbell'} {status} {next} {menu} {error}>
	<div class="flex min-h-0 flex-1 flex-col gap-3.5 overflow-y-auto overscroll-contain px-4 pt-5 pb-4">
		{#if plan}
			<p class="flex items-baseline gap-2.5 px-1">
				<b class="font-[family-name:var(--font-digits)] text-[76px] leading-none font-bold">{plan}</b>
				<span class="text-[15px] font-semibold text-(--fg2)">
					{[sides === 2 ? 'per side' : '', lastWeight !== null ? `${lastWeight} kg last time` : ''].filter(Boolean).join(' · ')}
				</span>
			</p>
		{/if}

		{#if !finished}
			<RestCard bind:this={rest} runId={openRun.run!.id} seconds={step.set_rest_seconds} offer={logged.length > 0} />
		{/if}

		<ol class="overflow-hidden rounded-[18px] bg-(--glass)">
			{#each done as pair, i (i)}
				<li class="[&:not(:first-child)]:shadow-[inset_0_1px_0_rgba(255,255,255,0.07)]">
					<button type="button" class="flex h-[78px] w-full items-center gap-3 px-4 text-left text-lg font-bold" aria-expanded={editing === i} onclick={() => edit(i)}>
						{#if pair.length === sides}{@render tick()}{:else}{@render ring()}{/if}
						Set {i + 1}
						<span class="ml-auto truncate text-[17px] font-semibold text-(--fg2)">
							{#if sides === 2}
								{pair.map((s) => `${side(s.number)[0]} ${short(s)}`).join(' · ')}
							{:else}
								{short(pair[0])}
							{/if}
						</span>
					</button>
					{#if editing === i}
						<div class="flex flex-col gap-2 px-3 pb-3">
							{#each draft as _, k (k)}
								{#if sides === 2}<p class="px-1 text-xs font-semibold text-(--fg2)">{side(i * 2 + k + 1)}</p>{/if}
								<div class="grid grid-cols-2 gap-2">
									<Stepper label="Reps" bind:value={draft[k].reps} />
									<Stepper label="kg" step={2.5} min={-200} placeholder="–" bind:value={draft[k].weight_kg} />
								</div>
							{/each}
						</div>
					{/if}
				</li>
			{/each}
			{#if !finished}
				<li class="bg-tint/7 px-4 py-4 [&:not(:first-child)]:shadow-[inset_0_1px_0_rgba(255,255,255,0.07)] {editing !== null ? 'opacity-50' : ''}">
					<p class="flex items-center gap-3 text-lg font-bold">
						{@render ring()}Set {number}{sides === 2 ? ` · ${side(row)}` : ''}
					</p>
					{#if editing === null}
						<div class="mt-3 grid grid-cols-2 gap-2">
							<Stepper label="Reps" bind:value={reps} />
							<Stepper label="kg" step={2.5} min={-200} placeholder="–" bind:value={weight} />
						</div>
					{/if}
				</li>
			{/if}
			{#each later as n (n)}
				<li class="flex h-[78px] items-center gap-3 px-4 text-lg font-bold text-(--fg2) [&:not(:first-child)]:shadow-[inset_0_1px_0_rgba(255,255,255,0.07)]">
					{@render ring()}Set {n}
					{#if step.reps !== null}<span class="ml-auto text-[17px] font-semibold">{step.reps} reps{sides === 2 ? ' each side' : ''}</span>{/if}
				</li>
			{/each}
		</ol>
	</div>

	{#snippet buttons()}
		{#if editing !== null}
			<button type="button" class="run-btn bg-(--glass)" onclick={() => (editing = null)}>Cancel</button>
			<button type="button" class="run-btn bg-tint text-on-tint" onclick={saveEdit}>
				<span class="run-icon"><Icon name="check" size="16px" stroke={3} /></span>Save set {editing + 1}
			</button>
		{:else if finished}
			<a href={nextHref} class="run-btn bg-live text-on-live">
				{last ? 'Back to session' : 'Next exercise'}<span class="run-icon"><Icon name="arrow-right" size="16px" stroke={2.6} /></span>
			</a>
		{:else}
			<button type="button" class="run-btn bg-tint text-on-tint" onclick={log}>
				<span class="run-icon"><Icon name="check" size="16px" stroke={3} /></span>Log {sides === 2 ? side(row).toLowerCase() : `set ${number}`}
			</button>
			<button type="button" class="run-round" aria-label="Skip" onclick={skip}><Icon name="skip" size="26px" /></button>
		{/if}
	{/snippet}
</RunPage>
