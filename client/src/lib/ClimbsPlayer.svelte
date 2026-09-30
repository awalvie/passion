<script lang="ts">
	import Button from './Button.svelte';
	import { loadGrades, scaleFor, type Grades } from './grades';
	import { newId } from './id';
	import type { Account } from './plan';
	import { climbsOf, isFinished, type Climb, type Discipline, type Outcome, type RunStep } from './run';
	import { openRun } from './runState.svelte';
	import Stepper from './Stepper.svelte';

	let { step }: { step: RunStep } = $props();

	const logged = $derived(climbsOf(openRun.run!, step.id));

	let info = $state<{ grades: Grades; account: Account } | null>(null);
	$effect(() => {
		loadGrades()
			.then((g) => (info = g))
			.catch(() => {});
	});

	// The bar sets every new climb, and starts from the last one logged.
	const last = $derived(logged.at(-1));
	let discipline = $state<Discipline>('boulder');
	let setting = $state<'indoor' | 'outdoor'>('indoor');
	let ropeStyle = $state('lead');
	$effect.pre(() => {
		if (!last) return;
		discipline = last.discipline;
		setting = last.setting;
		ropeStyle = last.rope_style ?? 'lead';
	});

	const boulder = $derived(discipline === 'boulder');
	const scale = $derived(info ? scaleFor(info.grades, info.account, boulder) : undefined);

	let grade = $state('');
	let outcome = $state<Outcome>('flash');
	let attempts = $state<number | null>(1);

	const outcomes: { value: Outcome; label: string; route?: boolean }[] = [
		{ value: 'onsight', label: 'Onsight', route: true },
		{ value: 'flash', label: 'Flash' },
		{ value: 'redpoint', label: 'Send' },
		{ value: 'hangdog', label: 'Hangdog', route: true },
		{ value: 'working', label: 'Working' }
	];

	function log() {
		const ungraded = info?.grades.ungraded.includes(grade);
		openRun.putClimb(step, newId(), {
			step: step.id,
			position: logged.length,
			discipline,
			setting,
			board: null,
			rope_style: boulder ? null : ropeStyle,
			grade: grade || null,
			grade_system: grade && !ungraded ? (scale?.system ?? null) : null,
			outcome,
			attempts: attempts === null ? null : Math.max(1, Math.round(attempts)),
			seconds: null,
			stars: null,
			focus: null,
			notes: null
		});
		attempts = 1;
	}

	function describe(c: Climb) {
		const label = outcomes.find((o) => o.value === c.outcome)?.label;
		const tries = c.attempts ? `${c.attempts} ${c.attempts === 1 ? 'try' : 'tries'}` : '';
		return [label, tries].filter(Boolean).join(' · ');
	}
</script>

{#snippet choice<T extends string>(options: { value: T; label: string }[], value: T, pick: (v: T) => void)}
	<div class="flex gap-1 rounded-xl bg-surface p-1 shadow-sm">
		{#each options as o (o.value)}
			<button
				type="button"
				class="h-9 flex-1 rounded-lg text-sm {value === o.value ? 'bg-tint font-semibold text-on-tint' : 'text-ink'}"
				aria-pressed={value === o.value}
				onclick={() => pick(o.value)}
			>
				{o.label}
			</button>
		{/each}
	</div>
{/snippet}

<p class="text-base text-ink-2">{logged.length} {logged.length === 1 ? 'climb' : 'climbs'} logged</p>

<div class="flex flex-col gap-2">
	{@render choice(
		[
			{ value: 'boulder', label: 'Boulder' },
			{ value: 'sport', label: 'Sport' },
			{ value: 'trad', label: 'Trad' }
		],
		discipline,
		(v) => (discipline = v)
	)}
	{@render choice(
		[
			{ value: 'indoor', label: 'Indoor' },
			{ value: 'outdoor', label: 'Outdoor' }
		],
		setting,
		(v) => (setting = v)
	)}
	{#if !boulder}
		{@render choice(
			[
				{ value: 'lead', label: 'Lead' },
				{ value: 'top_rope', label: 'Top rope' },
				{ value: 'auto_belay', label: 'Auto belay' },
				{ value: 'follow', label: 'Follow' }
			],
			ropeStyle,
			(v) => (ropeStyle = v)
		)}
	{/if}
</div>

{#if logged.length}
	<ol class="overflow-hidden rounded-2xl bg-surface shadow-sm">
		{#each logged as c, i (c.id)}
			<li class="flex items-center justify-between gap-3 border-line px-4 py-3 text-base [&:not(:first-child)]:border-t">
				<span class="text-ink-2">Climb {i + 1}</span>
				<span class="font-semibold">{c.grade ?? 'No grade'}</span>
				<span class="ml-auto text-ink-2">{describe(c)}</span>
			</li>
		{/each}
	</ol>
{/if}

{#if !isFinished(step)}
	<section class="flex flex-col gap-4 rounded-2xl bg-surface p-4 shadow-sm">
		<p class="text-center text-base font-semibold">Climb {logged.length + 1}</p>
		<div class="flex flex-wrap gap-2">
			{#each outcomes.filter((o) => !o.route || !boulder) as o (o.value)}
				<button
					type="button"
					class="h-10 rounded-full px-4 text-base {outcome === o.value ? 'bg-tint font-semibold text-on-tint' : 'bg-ground text-ink'}"
					aria-pressed={outcome === o.value}
					onclick={() => (outcome = o.value)}
				>
					{o.label}
				</button>
			{/each}
		</div>
		<div class="grid grid-cols-2 items-end gap-2">
			<label class="flex flex-col gap-1 text-sm text-ink-2">
				Grade{scale ? ` (${scale.system})` : ''}
				<select class="input" bind:value={grade}>
					<option value="">No grade</option>
					{#each scale?.grades ?? [] as g (g)}
						<option value={g}>{g}</option>
					{/each}
					{#each info?.grades.ungraded ?? [] as g (g)}
						<option value={g}>{g}</option>
					{/each}
				</select>
			</label>
			<Stepper label="Tries" min={1} bind:value={attempts} />
		</div>
		<Button onclick={log}>Log climb {logged.length + 1}</Button>
	</section>
{/if}
