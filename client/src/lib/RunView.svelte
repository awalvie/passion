<script lang="ts">
	import type { Snippet } from 'svelte';
	import { fullDate, timeOfDay } from './dates';
	import { formatDuration, formatMinutes } from './exercise';
	import Icon from './Icon.svelte';
	import { climbTally } from './progress';
	import Section from './Section.svelte';
	import { boards, climbsOf, climbWord, disciplines, ropes, settings, setsOf, stepTime, type LoggedSet, type Run, type RunStep } from './run';
	import { plural } from './text';

	// A finished session as logged: its journal, then each section's steps.
	// exerciseHref says where an exercise's name goes; actions sit beside the title.
	let {
		run,
		heading = 'h1',
		exerciseHref,
		actions
	}: { run: Run; heading?: 'h1' | 'h2'; exerciseHref: (s: RunStep) => string; actions?: Snippet } = $props();

	// The journal form is a page of its own, even from a pane whose links keep
	// their place.
	const away = { 'data-sveltekit-replacestate': 'false', 'data-sveltekit-noscroll': 'false' };

	// The end follows the session clock, so it matches the length beside it
	// even when the person ended the session late.
	const ended = $derived(
		run.elapsed_seconds === null ? null : new Date(Date.parse(run.started_at) + run.elapsed_seconds * 1000).toISOString()
	);
	const when = $derived(
		[
			fullDate(run.local_date),
			[run.started_at, ended].flatMap((t) => (t ? [timeOfDay(t, run.timezone)] : [])).join('–'),
			run.elapsed_seconds !== null && formatMinutes(run.elapsed_seconds)
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

	// A ladder set reads 3/6/9 s, as its times already count the reps.
	function work(s: LoggedSet) {
		const ladder = s.rep_seconds?.length ? `${s.rep_seconds.join('/')} s` : null;
		return [!ladder && s.reps !== null && plural(s.reps, 'rep'), ladder ?? (s.seconds !== null && `${s.seconds} s`)]
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
			const c = climbs[0];
			if (!c) return 'No climbs';
			const graded = climbs.filter((c) => c.grade_system).length;
			const sent = climbs.filter((c) => c.sent).length;
			return [
				disciplines[c.discipline],
				settings[c.setting],
				c.discipline === 'boulder'
					? c.setting === 'indoor' && c.board && c.board !== 'none' && boards[c.board as keyof typeof boards]
					: c.rope_style && ropes[c.rope_style as keyof typeof ropes],
				`${climbTally(climbs)}${graded ? `, ${sent} sent` : ''}`
			]
				.filter(Boolean)
				.join(' · ');
		}
		const n = setsOf(run, s.id).length;
		return n ? plural(n, 'set') : 'Nothing logged';
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

<div class="flex flex-col gap-7">
	<header class="flex items-start gap-3">
		<div class="min-w-0 flex-1 px-1">
			<svelte:element this={heading} class="page-title text-balance">{run.name}</svelte:element>
			<p class="mt-2 text-[15px] font-semibold text-ink-2">{when}</p>
		</div>
		{@render actions?.()}
	</header>

	{#if !scores.length && !words.length && !thoughts.length}
		<a href="/run/{run.id}/finish" {...away} class="flex min-h-16 items-center gap-3 rounded-3xl bg-surface px-4 py-3 shadow-card">
			<span class="flex size-9 shrink-0 items-center justify-center rounded-full bg-well text-ink-2"><Icon name="pencil" size="17px" stroke={2.2} /></span>
			<span class="flex-1 text-[15px] font-bold">Add session journal</span>
			<span class="text-ink-3"><Icon name="chevron-right" size="18px" stroke={2.4} /></span>
		</a>
	{:else}
		<Section title="Session journal">
			<!-- The whole card opens the journal form; the first row leaves room for the pencil. -->
			<a href="/run/{run.id}/finish" {...away} class="relative flex flex-col gap-4 rounded-3xl bg-surface px-[18px] pt-4 pb-[18px] shadow-card">
				<span class="absolute top-3.5 right-3.5 flex size-9 items-center justify-center rounded-full bg-well text-ink-2"><Icon name="pencil" size="16px" stroke={2.2} /><span class="sr-only">Edit session journal</span></span>
				{#each thoughts as [label, text], i (label)}
					<div class="flex flex-col gap-1 {i ? '' : 'pr-11'}">
						{#if label}<span class="text-xs font-bold text-ink-2">{label}</span>{/if}
						{@render note(text)}
					</div>
				{/each}
				{#if scores.length || words.length}
					<div class="flex flex-col gap-3.5 {thoughts.length ? 'mt-0.5 pt-4 shadow-[inset_0_1px_0_var(--line)]' : 'pr-11'}">
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
			</a>
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
						{@const sets = s.kind === 'reps_and_sets' || s.kind === 'timed_reps' ? setsOf(run, s.id) : []}
						{@const climbs = s.kind === 'climbing' ? climbsOf(run, s.id) : []}
						<article class="rounded-3xl bg-surface px-[18px] py-4 shadow-card">
							<div class="flex items-baseline gap-3">
								<div class="min-w-0 flex-1">
									<h3 class="text-[15px] font-bold">
										{#if s.kind === 'open'}
											{s.name}
										{:else}
											<a
												href={exerciseHref(s)}
												class="relative inline-flex items-center gap-0.5 before:absolute before:-inset-x-1 before:-inset-y-3 before:content-['']"
												>{s.name}<span class="text-ink-3"><Icon name="chevron-right" size="15px" stroke={2.6} /></span></a
											>
										{/if}
									</h3>
									{#if info}<p class="mt-0.5 text-xs font-semibold text-ink-2">{info}</p>{/if}
								</div>
								{#if time}<span class="shrink-0">{@render amount(time)}</span>{/if}
							</div>
							{#if sets.length}
								<ul class="mt-2.5">
									{#each sets as set (set.number)}
										<li class="flex items-baseline gap-3.5 py-2 list-row">
											<span class="w-3 digits text-[15px] text-ink-2">{set.number}</span>
											<span class="flex-1">{@render amount(work(set))}</span>
											{#if set.weight_kg !== null}{@render amount(`${set.weight_kg} kg`)}{/if}
										</li>
									{/each}
								</ul>
							{/if}
							{#if climbs.length}
								<ol class="mt-1.5">
									{#each climbs as c (c.id)}
										<!-- An ungraded climb is never a send, so it says only its tries, and one try nothing. -->
										{@const result = [c.grade_system && climbWord(c), c.goes.length > 1 && `${c.goes.length} tries`].filter(Boolean).join(' · ')}
										{#if c.grade || result || c.seconds || c.focus || c.notes}
											<li class="pt-3 list-row [&:not(:first-child)]:mt-3">
												{#if c.grade || result || c.seconds}
													<div class="flex items-baseline gap-3">
														{#if c.grade}
															<span class="min-w-12 {c.grade_system ? 'digits text-xl' : 'text-[15px] font-bold'}">{c.grade}</span>
														{/if}
														<span class="flex-1 text-xs font-bold {c.sent ? '' : 'text-ink-2'}">
															{#if c.sent}
																<span class="mr-1.5 inline-flex size-[18px] items-center justify-center rounded-full bg-rest-on-hero align-[-2px] text-on-tint">
																	<Icon name="check" size="12px" stroke={3} />
																</span>
															{/if}{result}
														</span>
														{#if c.seconds}{@render amount(formatDuration(c.seconds))}{/if}
													</div>
												{/if}
												{#if c.focus}
													<p class="mt-2.5 flex items-start first:mt-0 gap-2 text-[15px] leading-5 font-semibold text-ink-2">
														<span class="flex size-5 shrink-0 items-center justify-center rounded-full bg-well">
															<Icon name="target" size="0.75rem" stroke={2.4} />
														</span>
														{c.focus}
													</p>
												{/if}
												{#if c.notes}{@render note(c.notes, 'mt-2.5 first:mt-0')}{/if}
											</li>
										{/if}
									{/each}
								</ol>
							{/if}
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
