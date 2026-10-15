<script lang="ts">
	import { formatDate, timeOfDay } from '$lib/dates';
	import { formatDuration } from '$lib/exercise';
	import Icon from '$lib/Icon.svelte';
	import NavBar from '$lib/NavBar.svelte';
	import Section from '$lib/Section.svelte';
	import { climbsOf, setsOf, stepTime, type LoggedSet, type RunStep } from '$lib/run';

	let { data } = $props();

	const run = $derived(data.run);

	// The end follows the session clock, so it matches the length beside it
	// even when the person ended the session late.
	const ended = $derived(
		run.elapsed_seconds === null ? null : new Date(Date.parse(run.started_at) + run.elapsed_seconds * 1000).toISOString()
	);
	const when = $derived(
		[
			formatDate(run.local_date, { weekday: 'long', day: 'numeric', month: 'long' }),
			[run.started_at, ended].flatMap((t) => (t ? [timeOfDay(t, run.timezone)] : [])).join('–'),
			run.elapsed_seconds !== null && formatDuration(Math.max(1, Math.round(run.elapsed_seconds / 60)) * 60)
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
			['Where', [run.setting, run.place].filter(Boolean).join(', ')]
		].filter(([, v]) => v) as [string, string][]
	);

	const thoughts = $derived(
		[
			['What went well', run.went_well],
			['Next time', run.next_focus],
			['', run.notes]
		].filter(([, v]) => v) as [string, string][]
	);

	// A ladder set reads 3/6/9s × 10 kg, as its times already count the reps.
	function setText(s: LoggedSet) {
		const ladder = s.rep_seconds?.length ? `${s.rep_seconds.join('/')}s` : null;
		return [
			!ladder && s.reps !== null && `${s.reps}`,
			ladder ?? (s.seconds !== null && `${s.seconds}s`),
			s.weight_kg !== null && `${s.weight_kg} kg`
		]
			.filter(Boolean)
			.join(' × ');
	}

	// A step never done reads as one grey line, which runs of them share.
	type Block = { step: RunStep } | { label: string; names: string[] };
	function blocks(steps: RunStep[]): Block[] {
		const out: Block[] = [];
		for (const s of steps) {
			if (s.status === 'done') {
				out.push({ step: s });
				continue;
			}
			const label = s.status === 'skipped' ? 'Skipped' : 'Not done';
			const last = out.at(-1);
			if (last && 'label' in last && last.label === label) last.names.push(s.name);
			else out.push({ label, names: [s.name] });
		}
		return out;
	}

	function meta(s: RunStep) {
		if (s.kind === 'open') return '';
		if (s.kind === 'climbing') {
			const climbs = climbsOf(run, s.id);
			return climbs.map((c) => `${c.grade ?? '?'}${c.sent ? ' ✓' : ''}`).join(', ') || 'No climbs';
		}
		return setsOf(run, s.id).map(setText).join(', ') || 'Nothing logged';
	}
</script>

<!-- A value as 18m 30s, with the numbers bold and the units small. -->
{#snippet amount(text: string)}
	<span class="text-xs font-semibold text-ink-2">
		{#each text.split(/([\d.]+)/) as part, i (i)}
			{#if i % 2}<span class="mr-0.5 digits text-xl text-ink">{part}</span>{:else}{part}{/if}
		{/each}
	</span>
{/snippet}

<!-- Notes as typed, with blank lines turned into paragraph gaps. -->
{#snippet note(text: string, cls = '')}
	<div class="flex flex-col gap-2.5 reading {cls}">
		{#each text.split(/\n\s*\n/) as p, i (i)}<p>{p.trim()}</p>{/each}
	</div>
{/snippet}

<svelte:head><title>{run.name}</title></svelte:head>

<NavBar back={{ href: '/history', label: 'History' }}>
	{#snippet actions()}
		<a href="/run/{run.id}" class="flex h-11 items-center rounded-full bg-surface px-4 text-[15px] font-bold text-ink shadow-card-sm">Edit</a>
	{/snippet}
</NavBar>

<div class="flex flex-col gap-7 px-4 pt-2">
	<header class="px-1">
		<h1 class="page-title text-balance">{run.name}</h1>
		<p class="mt-2 text-[15px] font-semibold text-ink-2">{when}</p>
	</header>

	{#if scores.length || words.length || thoughts.length}
		<Section title="Journal">
			<div class="flex flex-col gap-4 rounded-3xl bg-surface px-[18px] pt-4 pb-[18px] shadow-card">
				{#each thoughts as [label, text] (label)}
					<div class="flex flex-col gap-1">
						{#if label}<span class="text-xs font-bold text-ink-2">{label}</span>{/if}
						{@render note(text)}
					</div>
				{/each}
				{#if scores.length || words.length}
					<div class="flex flex-col gap-3.5 {thoughts.length ? 'mt-0.5 pt-4 shadow-[inset_0_1px_0_var(--line)]' : ''}">
						{#if scores.length}
							<dl class="grid grid-cols-3 gap-4">
								{#each scores as [label, value, max] (label)}
									<div class="flex flex-col gap-1">
										<dt class="text-xs font-semibold text-ink-2">{label}</dt>
										<dd class="flex flex-col gap-1">
											<span class="flex items-baseline gap-1"><span class="digits text-xl">{value}</span><span class="text-xs font-semibold text-ink-2">/ {max}</span></span>
											<span class="h-1.5 overflow-hidden rounded-full bg-well">
												<span class="block h-full rounded-full bg-ink-2" style="width: {(value! / max) * 100}%"></span>
											</span>
										</dd>
									</div>
								{/each}
							</dl>
						{/if}
						{#if words.length}
							<dl class="grid grid-cols-3 gap-4">
								{#each words as [label, value], i (label)}
									<div class={i === words.length - 1 ? 'col-span-2' : ''}>
										<dt class="text-xs font-semibold text-ink-2">{label}</dt>
										<dd class="mt-0.5 text-[15px] font-bold first-letter:uppercase">{value}</dd>
									</div>
								{/each}
							</dl>
						{/if}
					</div>
				{/if}
			</div>
		</Section>
	{/if}

	{#each run.sections as section, i (i)}
		{@const steps = section.items.flatMap((item) => (item.step ? [item.step] : []))}
		{#if steps.length}
			<Section title={section.name}>
				{#each blocks(steps) as b, j (j)}
					{#if 'step' in b}
						{@const s = b.step}
						{@const time = stepTime(run, s)}
						{@const info = meta(s)}
						<article class="rounded-3xl bg-surface px-[18px] py-4 shadow-card">
							<div class="flex items-baseline gap-3">
								<div class="min-w-0 flex-1">
									<h3 class="text-[15px] font-bold">{s.name}</h3>
									{#if info}<p class="mt-0.5 text-xs font-semibold text-ink-2">{info}</p>{/if}
								</div>
								{#if time}<span class="shrink-0">{@render amount(time)}</span>{/if}
							</div>
							{#if s.run_notes}{@render note(s.run_notes, 'mt-3')}{/if}
						</article>
					{:else}
						<p class="flex items-start gap-2.5 px-[18px] {j ? 'mt-1' : ''} text-xs leading-5 font-semibold text-ink-2">
							<span class="flex size-5 shrink-0 items-center justify-center rounded-full bg-well">
								<Icon name="minus" size="0.75rem" stroke={2.4} />
							</span>
							<span><b class="font-bold">{b.label}</b> · {b.names.join(', ')}</span>
						</p>
					{/if}
				{/each}
			</Section>
		{/if}
	{/each}
</div>
