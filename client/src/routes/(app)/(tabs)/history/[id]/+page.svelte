<script lang="ts">
	import NavBar from '$lib/NavBar.svelte';
	import { climbsOf, setsOf, stepsOf, type LoggedSet, type RunStep } from '$lib/run';

	let { data } = $props();

	const run = $derived(data.run);

	const date = $derived(
		new Date(`${run.local_date}T12:00:00`).toLocaleDateString(undefined, {
			weekday: 'long',
			day: 'numeric',
			month: 'long'
		})
	);

	const done = $derived(stepsOf(run).filter((s) => s.status === 'done').length);
	const facts = $derived(
		[
			run.elapsed_seconds !== null && `${Math.round(run.elapsed_seconds / 60)} min`,
			run.place,
			`${done} exercise${done === 1 ? '' : 's'}`
		]
			.filter(Boolean)
			.join(' · ')
	);

	const journal = $derived(
		[
			['Sleep', run.sleep && `${run.sleep} / 5`],
			['Energy', run.energy && `${run.energy} / 5`],
			['Effort', run.rpe && `${run.rpe} / 10`],
			['Focus', run.focus],
			['Where', run.setting]
		].filter(([, v]) => v) as [string, string][]
	);

	function setText(s: LoggedSet) {
		return [
			s.reps !== null && `${s.reps}`,
			s.seconds !== null && `${s.seconds}s`,
			s.weight_kg !== null && `${s.weight_kg} kg`
		]
			.filter(Boolean)
			.join(' × ');
	}

	function logText(s: RunStep) {
		if (s.status === 'skipped') return 'Skipped';
		if (s.kind === 'climbing') {
			const climbs = climbsOf(run, s.id);
			return climbs.map((c) => `${c.grade ?? '?'}${c.sent ? ' ✓' : ''}`).join(', ') || 'No climbs';
		}
		return setsOf(run, s.id).map(setText).join(', ') || 'Nothing logged';
	}
</script>

<svelte:head><title>{run.name}</title></svelte:head>

<NavBar title={run.name} back={{ href: '/history', label: 'History' }}>
	{#snippet actions()}
		<a href="/run/{run.id}" class="px-3 text-base text-tint">Edit</a>
	{/snippet}
</NavBar>

<div class="flex flex-col gap-4 px-4 pt-2">
	<header class="flex flex-col gap-1">
		<p class="text-sm text-ink-2">{date}</p>
		<h1 class="text-3xl font-bold">{run.name}</h1>
		<p class="text-base text-ink-2">{facts}</p>
	</header>

	{#if journal.length || run.went_well || run.next_focus || run.notes}
		<section class="flex flex-col gap-3 rounded-2xl bg-surface p-4 shadow-sm">
			{#if journal.length}
				<dl class="grid grid-cols-3 gap-3">
					{#each journal as [label, value] (label)}
						<div>
							<dt class="text-sm text-ink-2">{label}</dt>
							<dd class="text-base font-semibold capitalize">{value}</dd>
						</div>
					{/each}
				</dl>
			{/if}
			{#if run.went_well}
				<p class="text-base"><span class="text-ink-2">Went well:</span> {run.went_well}</p>
			{/if}
			{#if run.next_focus}
				<p class="text-base"><span class="text-ink-2">Next time:</span> {run.next_focus}</p>
			{/if}
			{#if run.notes}
				<p class="text-base">{run.notes}</p>
			{/if}
		</section>
	{/if}

	{#each run.sections as section, i (i)}
		{@const steps = section.items.flatMap((item) => (item.step ? [item.step] : []))}
		{#if steps.length}
			<section class="flex flex-col gap-2">
				<h2 class="px-1 text-sm font-semibold text-ink-2">{section.name}</h2>
				<ul class="overflow-hidden rounded-2xl bg-surface shadow-sm">
					{#each steps as s (s.id)}
						<li class="border-line px-4 py-3 [&:not(:first-child)]:border-t">
							<p class="text-base {s.status === 'skipped' ? 'text-ink-2' : ''}">{s.name}</p>
							<p class="text-sm text-ink-2">{logText(s)}</p>
							{#if s.run_notes}
								<p class="mt-1 text-sm">{s.run_notes}</p>
							{/if}
						</li>
					{/each}
				</ul>
			</section>
		{/if}
	{/each}
</div>
