<script lang="ts">
	import Button from './Button.svelte';
	import PlayerStepFrame from './PlayerStepFrame.svelte';
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

	const shown = $derived(outcomes.filter((o) => !o.route || !boulder));

	// A logged climb opens in the card, and saving rewrites it under its id.
	let editing = $state<Climb | null>(null);

	function edit(c: Climb) {
		editing = c;
		discipline = c.discipline;
		setting = c.setting;
		ropeStyle = c.rope_style ?? 'lead';
		grade = c.grade ?? '';
		outcome = c.outcome ?? 'flash';
		attempts = c.attempts;
	}

	// A boulder and a route use different scales, and a boulder has no onsight
	// or hangdog, so the server would refuse what was picked for the other.
	function setDiscipline(next: Discipline) {
		if ((next === 'boulder') !== boulder) grade = '';
		if (next === 'boulder' && (outcome === 'onsight' || outcome === 'hangdog')) outcome = 'flash';
		discipline = next;
	}

	function remove() {
		openRun.removeClimb(editing!.id);
		editing = null;
	}

	function log() {
		const ungraded = info?.grades.ungraded.includes(grade);
		const position = editing?.position ?? Math.max(-1, ...logged.map((c) => c.position)) + 1;
		openRun.putClimb(step, editing?.id ?? newId(), {
			step: step.id,
			position,
			discipline,
			setting,
			board: boulder ? (editing?.board ?? null) : null,
			rope_style: boulder ? null : ropeStyle,
			grade: grade || null,
			grade_system: grade && !ungraded ? (scale?.system ?? null) : null,
			outcome,
			attempts: attempts === null ? null : Math.max(1, Math.round(attempts)),
			seconds: editing?.seconds ?? null,
			stars: editing?.stars ?? null,
			focus: editing?.focus ?? null,
			notes: editing?.notes ?? null
		});
		editing = null;
		attempts = 1;
	}

	const gradeList = $derived(['', ...(scale?.grades ?? []), ...(info?.grades.ungraded ?? [])]);

	function stepGrade(by: number) {
		const i = Math.max(0, gradeList.indexOf(grade)) + by;
		grade = gradeList[Math.min(gradeList.length - 1, Math.max(0, i))];
	}

	function describe(c: Climb) {
		const label = outcomes.find((o) => o.value === c.outcome)?.label;
		const tries = c.attempts ? `${c.attempts} ${c.attempts === 1 ? 'try' : 'tries'}` : '';
		return [label, tries].filter(Boolean).join(' · ');
	}
</script>

{#snippet divider()}
	<i class="h-[22px] w-px shrink-0 bg-ink-3/30"></i>
{/snippet}

{#snippet outcomeButton(o: { value: Outcome; label: string })}
	<button
		type="button"
		class="h-12 rounded-[14px] text-[15px] font-bold shadow-card-sm {outcome === o.value ? 'bg-ink text-ground' : 'bg-surface text-ink'}"
		aria-pressed={outcome === o.value}
		onclick={() => (outcome = o.value)}
	>
		{o.label}
	</button>
{/snippet}

<div class="flex h-12 items-center rounded-full bg-well dark:bg-surface">
	<select
		class="input h-full flex-1 rounded-full border-0 bg-transparent bg-[position:right_0.5rem_center] px-2 pr-6 text-center text-[15px] font-bold text-ink"
		aria-label="Discipline"
		value={discipline}
		onchange={(e) => setDiscipline(e.currentTarget.value as Discipline)}
	>
		<option value="boulder">Boulder</option>
		<option value="sport">Sport</option>
		<option value="trad">Trad</option>
	</select>
	{@render divider()}
	<select
		class="input h-full flex-1 rounded-full border-0 bg-transparent bg-[position:right_0.5rem_center] px-2 pr-6 text-center text-[15px] font-bold text-ink"
		aria-label="Indoor or outdoor"
		bind:value={setting}
	>
		<option value="indoor">Indoor</option>
		<option value="outdoor">Outdoor</option>
	</select>
	{#if !boulder}
		{@render divider()}
		<select
			class="input h-full flex-1 rounded-full border-0 bg-transparent bg-[position:right_0.5rem_center] px-2 pr-6 text-center text-[15px] font-bold text-ink"
			aria-label="Rope"
			bind:value={ropeStyle}
		>
			<option value="lead">Lead</option>
			<option value="top_rope">Top rope</option>
			<option value="auto_belay">Auto belay</option>
			<option value="follow">Follow</option>
		</select>
	{/if}
</div>

{#if logged.length || editing || !isFinished(step)}
	<div class="rounded-3xl bg-surface py-0.5 shadow-card">
		{#if logged.length}
			<ol>
				{#each logged as c, i (c.id)}
					<li class="border-line [&:not(:first-child)]:border-t">
						<button
							type="button"
							class="flex h-11 w-full items-center gap-2.5 px-4 text-left text-[15px] {editing?.id === c.id ? 'bg-well' : ''}"
							onclick={() => edit(c)}
						>
							<span class="flex size-6 shrink-0 items-center justify-center rounded-full bg-ink text-xs font-bold text-ground dark:bg-ink-2">
								<span class="sr-only">Climb</span>{i + 1}
							</span>
							<span class="font-bold">{c.grade ?? 'No grade'}</span>
							<span class="min-w-0 flex-1 truncate font-semibold text-ink-2">{describe(c)}</span>
							<svg viewBox="0 0 24 24" class="size-4 shrink-0 text-ink-3" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M9 6l6 6-6 6" /></svg>
						</button>
					</li>
				{/each}
			</ol>
		{/if}

		{#if editing || !isFinished(step)}
			{@const number = editing ? logged.findIndex((c) => c.id === editing!.id) + 1 : logged.length + 1}
			<section class="mx-1.5 my-0.5 rounded-[20px] bg-well p-2.5 dark:bg-[#1F2721]">
				<p class="px-1 pb-2 text-[15px] font-bold">{editing ? 'Edit climb' : 'Climb'} {number}</p>
				<div class="grid grid-cols-3 gap-2">
					{#each shown.slice(0, 3) as o (o.value)}
						{@render outcomeButton(o)}
					{/each}
				</div>
				{#if shown.length > 3}
					<div class="mt-1.5 grid grid-cols-2 gap-2">
						{#each shown.slice(3) as o (o.value)}
							{@render outcomeButton(o)}
						{/each}
					</div>
				{/if}
				<div class="mt-2 grid grid-cols-2 gap-2">
					<PlayerStepFrame label="Grade{scale ? ` (${scale.system})` : ''}" chars={grade.length || 1} less={() => stepGrade(-1)} more={() => stepGrade(1)}>
						<span class="relative block w-full text-center">
							<span class="block truncate leading-[1.05] font-extrabold tracking-tight {grade ? 'text-ink' : 'text-ink-3'}" aria-hidden="true">
								{grade || '–'}
							</span>
							<select class="input absolute inset-0 h-full min-h-0 cursor-pointer opacity-0" aria-label="Grade" bind:value={grade}>
								<option value="">No grade</option>
								{#each scale?.grades ?? [] as g (g)}
									<option value={g}>{g}</option>
								{/each}
								{#each info?.grades.ungraded ?? [] as g (g)}
									<option value={g}>{g}</option>
								{/each}
							</select>
						</span>
					</PlayerStepFrame>
					<Stepper label="Tries" min={1} bind:value={attempts} />
				</div>
				<div class="mt-2.5">
					<Button onclick={log}>
						<svg viewBox="0 0 24 24" class="size-5" fill="none" stroke="currentColor" stroke-width="2.6" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M5 12.5l4.5 4.5L19 7" /></svg>
						{editing ? 'Save' : 'Log'} climb {number}
					</Button>
				</div>
				{#if editing}
					<div class="mt-2 grid grid-cols-2 gap-2">
						<Button variant="secondary" onclick={() => (editing = null)}>Cancel</Button>
						<Button variant="danger" onclick={remove}>Remove</Button>
					</div>
				{/if}
			</section>
		{/if}
	</div>
{/if}
