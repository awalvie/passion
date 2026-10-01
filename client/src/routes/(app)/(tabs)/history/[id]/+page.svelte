<script lang="ts">
	import Icon from '$lib/Icon.svelte';
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

	const scores = $derived(
		(
			[
				['Sleep', run.sleep, 5],
				['Energy', run.energy, 5],
				['Effort', run.rpe, 10]
			] as [string, number | null, number][]
		).filter(([, v]) => v)
	);

	const words = $derived(
		[
			['Focus', run.focus],
			['Where', run.setting]
		].filter(([, v]) => v) as [string, string][]
	);

	const thoughts = $derived(
		[
			['Went well', run.went_well],
			['Next time', run.next_focus],
			['', run.notes]
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

<NavBar title={run.name} heading={false} back={{ href: '/history', label: 'History' }}>
	{#snippet actions()}
		<a href="/run/{run.id}" class="flex h-10 items-center rounded-full bg-surface px-4 text-[15px] font-bold text-ink shadow-card-sm">Edit</a>
	{/snippet}
</NavBar>

<div class="flex flex-col gap-3.5 px-4 pt-2">
	<header class="px-1">
		<h1 class="text-[32px] leading-[1.1] font-extrabold tracking-[-0.02em]">{run.name}</h1>
		<p class="mt-1 text-[15px] font-semibold text-ink-2">{date}</p>
		<p class="mt-0.5 text-[15px] font-semibold text-ink-2">{facts}</p>
	</header>

	{#if scores.length || words.length || thoughts.length}
		<section class="flex flex-col gap-4 rounded-3xl bg-surface px-[18px] pt-4 pb-[18px] shadow-card">
			<h2 class="text-[15px] font-bold">Journal</h2>
			{#if scores.length}
				<dl class="grid grid-cols-3 gap-3">
					{#each scores as [label, value, max] (label)}
						<div class="flex flex-col gap-1">
							<dt class="text-xs font-semibold text-ink-2">{label}</dt>
							<dd class="flex flex-col gap-1">
								<span class="text-xl leading-none font-bold">{value}<span class="ml-1 text-xs font-semibold text-ink-2">/ {max}</span></span>
								<span class="h-1.5 overflow-hidden rounded-full bg-well">
									<span class="block h-full rounded-full bg-ink dark:bg-ink-2" style="width: {(value! / max) * 100}%"></span>
								</span>
							</dd>
						</div>
					{/each}
				</dl>
			{/if}
			{#if words.length}
				<dl class="grid grid-cols-3 gap-3">
					{#each words as [label, value] (label)}
						<div>
							<dt class="text-xs font-semibold text-ink-2">{label}</dt>
							<dd class="text-[15px] font-bold capitalize">{value}</dd>
						</div>
					{/each}
				</dl>
			{/if}
			{#each thoughts as [label, text] (label)}
				<div class="flex flex-col gap-1 rounded-[18px] bg-inset px-4 py-3.5 text-[15px] font-semibold">
					{#if label}<span class="text-xs font-semibold text-ink-2">{label}</span>{/if}
					{text}
				</div>
			{/each}
		</section>
	{/if}

	{#each run.sections as section, i (i)}
		{@const steps = section.items.flatMap((item) => (item.step ? [item.step] : []))}
		{#if steps.length}
			<section class="rounded-3xl bg-surface px-[18px] pt-4 pb-1 shadow-card">
				<h2 class="mb-1 text-[15px] font-bold">{section.name}</h2>
				<ul>
					{#each steps as s (s.id)}
						<li class="flex gap-3 py-3 [&:not(:first-child)]:shadow-[inset_0_1px_0_var(--line)]">
							{#if s.status === 'done'}
								<span class="mt-px flex size-6 shrink-0 items-center justify-center rounded-full bg-ink text-tint dark:bg-[#2A342D]">
									<Icon name="check" size="0.875rem" stroke={3} />
								</span>
							{:else}
								<span class="mt-px size-6 shrink-0 rounded-full bg-well"></span>
							{/if}
							<span class="min-w-0 flex-1">
								<span class="block text-[15px] font-bold {s.status === 'skipped' ? 'text-ink-2' : ''}">{s.name}</span>
								<span class="mt-0.5 block text-xs font-semibold text-ink-2">{logText(s)}</span>
								{#if s.run_notes}
									<span class="mt-1 block text-[15px] font-semibold">{s.run_notes}</span>
								{/if}
							</span>
						</li>
					{/each}
				</ul>
			</section>
		{/if}
	{/each}
</div>
