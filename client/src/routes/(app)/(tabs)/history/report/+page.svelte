<script lang="ts">
	import { goto } from '$app/navigation';
	import DateField from '$lib/DateField.svelte';
	import { daysBetween, dayDate, formatDate, timeOfDay } from '$lib/dates';
	import Empty from '$lib/Empty.svelte';
	import { formatMinutes } from '$lib/exercise';
	import NavBar from '$lib/NavBar.svelte';
	import { climbTally, measure, setLines } from '$lib/progress';
	import { boards, climbsOf, climbWord, disciplines, ropes, settings, setsOf, stepsOf, stepTime, type Run, type RunStep } from '$lib/run';
	import { plural } from '$lib/text';

	let { data } = $props();

	// A changed day reloads the page on the new range, in place.
	function pick(key: 'from' | 'to', v: string) {
		if (!v) return;
		const q = new URLSearchParams({ from: data.from, to: data.to, [key]: v });
		void goto(`?${q}`, { replaceState: true, noScroll: true, keepFocus: true });
	}

	const long = (d: string) => formatDate(d, { day: 'numeric', month: 'short', year: 'numeric' });
	const range = $derived(`${long(data.from)} – ${long(data.to)}`);
	const seconds = $derived(data.runs.reduce((n, r) => n + (r.elapsed_seconds ?? 0), 0));
	const weeks = $derived((daysBetween(data.from, data.to) + 1) / 7);
	const summary = $derived(
		[
			plural(data.runs.length, 'session'),
			seconds > 0 && formatMinutes(seconds),
			weeks >= 1 && data.runs.length > 0 && `${Math.round((data.runs.length / weeks) * 10) / 10} a week`
		]
			.filter(Boolean)
			.join(' · ')
	);

	// When, how long, and the journal's scores and words, on one line.
	function about(r: Run) {
		const ended = r.elapsed_seconds === null ? null : new Date(Date.parse(r.started_at) + r.elapsed_seconds * 1000).toISOString();
		return [
			[r.started_at, ended].flatMap((t) => (t ? [timeOfDay(t, r.timezone)] : [])).join('–'),
			r.sleep && `Sleep ${r.sleep}/5`,
			r.energy && `Energy ${r.energy}/5`,
			r.rpe && `Effort ${r.rpe}/10`,
			[r.setting, r.place].filter(Boolean).join(', ')
		]
			.filter(Boolean)
			.join(' · ');
	}

	const thoughts = (r: Run) =>
		(
			[
				['Focus', r.focus],
				['What went well', r.went_well],
				['Next time', r.next_focus],
				['Notes', r.notes]
			] as [string, string | null][]
		).filter(([, v]) => v) as [string, string][];

	// What a step's climbs were: where, then each climb's grade and result.
	function climbsAbout(r: Run, s: RunStep) {
		const cs = climbsOf(r, s.id);
		const c = cs[0];
		if (!c) return '';
		const graded = cs.filter((x) => x.grade_system).length;
		return [
			disciplines[c.discipline],
			settings[c.setting],
			c.discipline === 'boulder'
				? c.setting === 'indoor' && c.board && c.board !== 'none' && boards[c.board as keyof typeof boards]
				: c.rope_style && ropes[c.rope_style as keyof typeof ropes],
			typeof c.angle === 'number' && `${c.angle}°`,
			`${climbTally(cs)}${graded ? `, ${cs.filter((x) => x.sent).length} sent` : ''}`
		]
			.filter(Boolean)
			.join(' · ');
	}

	const skipped = (r: Run) => stepsOf(r).filter((s) => s.status !== 'done');

	// Paper is light whatever the theme.
	$effect(() => {
		const root = document.documentElement;
		let was = '';
		const before = () => {
			was = root.dataset.theme ?? '';
			root.dataset.theme = 'light';
		};
		const after = () => was && (root.dataset.theme = was);
		addEventListener('beforeprint', before);
		addEventListener('afterprint', after);
		return () => {
			removeEventListener('beforeprint', before);
			removeEventListener('afterprint', after);
		};
	});
</script>

<svelte:head><title>Training {range}</title></svelte:head>

<!-- A figure: its numbers in the number face, its words small and quiet. -->
{#snippet amount(text: string)}
	<span class="text-xs font-semibold text-ink-2">
		{#each text.split(/([\d.:/]+)/) as part, i (i)}
			{#if i % 2}<span class="digits text-[15px] font-semibold text-ink">{part}</span>{:else}{part}{/if}
		{/each}
	</span>
{/snippet}

<div class="mx-auto max-w-[760px] print:max-w-none">
<div class="contents print:hidden">
	<NavBar back={{ href: '/history', label: 'History' }} title="Export">
		{#snippet actions()}
			<button type="button" class="flex h-11 items-center rounded-full bg-tint px-5 text-[15px] font-bold text-on-tint shadow-card-sm" onclick={() => print()}>Print</button>
		{/snippet}
	</NavBar>
</div>

<div class="flex flex-col gap-3.5 px-4 pt-2 pb-8 print:p-0">
	<div class="flex flex-col gap-2 print:hidden">
		<div class="grid grid-cols-2 gap-2.5">
			<DateField label="From" max={data.to} pill bind:value={() => data.from, (v) => pick('from', v)} />
			<DateField label="To" min={data.from} max={data.today} pill bind:value={() => data.to, (v) => pick('to', v)} />
		</div>
		<p class="px-1 text-xs font-semibold text-ink-2">Print, then save it as a PDF to send it.</p>
	</div>

	<article class="rounded-3xl bg-surface px-[18px] pt-[18px] pb-2 shadow-card print:rounded-none print:bg-transparent print:p-0 print:shadow-none">
		<header class="pb-3">
			<h1 class="text-xl font-bold">Training log</h1>
			<p class="mt-0.5 text-[15px] font-semibold text-ink-2">{range}</p>
			<p class="mt-1 text-xs font-semibold text-ink-2">{summary}</p>
		</header>

		{#if !data.runs.length}
			<div class="border-t border-line py-4"><Empty title="No sessions" text="Pick other days to see their sessions." /></div>
		{/if}

		{#each data.runs as r (r.id)}
			{@const off = skipped(r)}
			<section class="break-inside-avoid border-t border-line py-4">
				<div class="flex items-baseline justify-between gap-3">
					<h2 class="min-w-0 text-[15px] font-bold">{dayDate(r.local_date)} · {r.name}</h2>
					{#if r.elapsed_seconds !== null}<span class="shrink-0">{@render amount(formatMinutes(r.elapsed_seconds))}</span>{/if}
				</div>
				<p class="mt-0.5 text-xs font-semibold text-ink-2">{about(r)}</p>

				{#each thoughts(r) as [label, text] (label)}
					<p class="mt-2 reading text-[15px]"><span class="mr-1 text-xs font-bold text-ink-2">{label}</span>{text}</p>
				{/each}

				<div class="mt-3 grid gap-x-4 gap-y-2.5 sm:grid-cols-[11rem_minmax(0,1fr)]">
					{#each stepsOf(r).filter((s) => s.status === 'done') as s (s.id)}
						{@const sets = setsOf(r, s.id)}
						{@const lines = s.kind === 'reps_and_sets' || s.kind === 'timed_reps' ? setLines(sets, s.per_side ? 2 : 1, measure([{ sets }]) === 'kg') : []}
						{@const climbs = s.kind === 'climbing' ? climbsOf(r, s.id) : []}
						{@const time = stepTime(r, s)}
						<div class="flex flex-col gap-1 sm:contents">
						<h3 class="text-[15px] font-semibold">{s.name}{#if s.per_side && lines.length}<span class="ml-1.5 text-xs text-ink-2">each side</span>{/if}</h3>
						<div class="flex flex-col gap-1">
							{#each lines as l (l.from)}
								<p class="flex items-baseline gap-2">
									<span class="w-9 shrink-0 digits text-xs font-semibold text-ink-2">{l.from === l.to ? l.from : `${l.from}–${l.to}`}</span>
									<span>{@render amount([l.work, l.load].filter(Boolean).join(' · '))}</span>
								</p>
							{/each}
							{#if climbs.length}
								<p class="text-xs font-semibold text-ink-2">{climbsAbout(r, s)}</p>
								<p class="flex flex-wrap gap-x-3.5 gap-y-1">
									{#each climbs as c (c.id)}
										{@const result = [c.grade_system && climbWord(c), c.goes.length > 1 && `${c.goes.length} tries`].filter(Boolean).join(', ')}
										<span class="whitespace-nowrap">
											{#if c.grade}<span class="digits text-[15px] font-semibold">{c.grade}</span>{/if}
											{#if result}<span class="text-xs font-semibold text-ink-2">{result}</span>{/if}
										</span>
									{/each}
								</p>
								{#each climbs.filter((c) => c.focus || c.notes) as c (c.id)}
									<p class="reading text-[15px]">
										{#if c.grade}<span class="mr-1 text-xs font-bold text-ink-2">{c.grade}</span>{/if}{[c.focus, c.notes].filter(Boolean).join('. ')}
									</p>
								{/each}
							{:else if time}
								<span>{@render amount(time)}</span>
							{:else if !lines.length}
								<span class="text-xs font-semibold text-ink-2">Nothing logged</span>
							{/if}
							{#if s.run_notes}<p class="reading text-[15px]">{s.run_notes}</p>{/if}
						</div>
						</div>
					{/each}
				</div>

				{#if off.length}
					<p class="mt-2.5 text-xs font-semibold text-ink-2"><span class="font-bold">Not done</span> · {off.map((s) => s.name).join(', ')}</p>
				{/if}
			</section>
		{/each}
	</article>
</div>
</div>
