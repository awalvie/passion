<script lang="ts">
	import { afterNavigate, goto } from '$app/navigation';
	import { page } from '$app/state';
	import { untrack } from 'svelte';
	import { MediaQuery } from 'svelte/reactivity';
	import { describe, request } from '$lib/api';
	import Bars from '$lib/Bars.svelte';
	import Button from '$lib/Button.svelte';
	import { formatDate, weekday } from '$lib/dates';
	import Empty from '$lib/Empty.svelte';
	import { formatMinutes } from '$lib/exercise';
	import Dots from '$lib/Dots.svelte';
	import ExerciseHistory from '$lib/ExerciseHistory.svelte';
	import FormError from '$lib/FormError.svelte';
	import Icon from '$lib/Icon.svelte';
	import { counts, gradeScale, grades, hours, isBoulder } from '$lib/progress';
	import { loadExerciseHistory, type Run, type RunSummary } from '$lib/run';
	import { openRun } from '$lib/runState.svelte';
	import RunView from '$lib/RunView.svelte';
	import SessionJournalForm from '$lib/SessionJournalForm.svelte';
	import { plural } from '$lib/text';

	let { data } = $props();

	// From lg a session opens in a pane beside the list, named by open; with
	// none named, the newest opens. An exercise's name in it swaps the pane for
	// that exercise's history, named by exercise.
	const wide = new MediaQuery('min-width: 64rem');
	const openId = $derived(wide.current ? page.url.searchParams.get('open') || data.runs[0]?.id : undefined);
	const exerciseId = $derived(wide.current ? page.url.searchParams.get('exercise') : null);
	const exerciseName = $derived(page.url.searchParams.get('name'));
	// journal swaps the pane for the session's journal form, which edits the
	// run through openRun as the form does in a session.
	const journaling = $derived(!!openId && page.url.searchParams.has('journal'));
	let opened = $state<Run | null>(null);
	let exercise = $state<(Awaited<ReturnType<typeof loadExerciseHistory>> & { id: string }) | null>(null);
	let paneError = $state('');

	// The pane keeps its own scroll, so another session or exercise starts it
	// at the top.
	let pane = $state<HTMLElement>();
	$effect(() => {
		void [openId, exerciseId, journaling];
		if (pane) pane.scrollTop = 0;
	});

	// A session opened from another page may sit far down the list. The newest,
	// open when none is named, is already in view.
	afterNavigate(({ from }) => {
		if (from?.route.id !== page.route.id && page.url.searchParams.has('open'))
			document.querySelector('[aria-current="true"]')?.scrollIntoView({ block: 'center' });
	});

	// Each fetch drops its answer when the pane has moved on before it came.
	function fill<T>(fetch: () => Promise<T>, done: (v: T) => void) {
		let gone = false;
		paneError = '';
		fetch().then(
			(v) => !gone && done(v),
			(e) => !gone && (paneError = describe(e, (f) => f))
		);
		return () => (gone = true);
	}
	$effect(() => {
		const id = openId;
		if (id) return fill(() => request<Run>('GET', `/api/v1/runs/${id}`), (r) => (opened = r));
	});
	$effect(() => {
		const id = exerciseId;
		const name = exerciseName;
		if (id) return fill(() => loadExerciseHistory(id, name), (x) => (exercise = { ...x, id }));
	});
	$effect(() => {
		const id = openId;
		// load reads the open run, which it then replaces.
		if (journaling && id) return untrack(() => fill(() => openRun.load(id), () => {}));
	});

	const months = $derived.by(() => {
		const out: { label: string; runs: RunSummary[] }[] = [];
		for (const r of data.runs) {
			const label = formatDate(r.local_date, { month: 'long', year: 'numeric' });
			if (out.at(-1)?.label !== label) out.push({ label, runs: [] });
			out.at(-1)!.runs.push(r);
		}
		return out;
	});

	function facts(r: RunSummary) {
		const length = r.elapsed_seconds === null ? '' : formatMinutes(r.elapsed_seconds);
		return [length, r.place].filter(Boolean).join(' · ');
	}

	const time = $derived(hours(data.runs, data.weeks));
	const climbs = $derived(counts(data.climbs, data.weeks));
	// The thumbnail follows whichever of boulders and routes was climbed more.
	const main = $derived.by(() => {
		const boulders = data.climbs.filter(isBoulder);
		const mine = boulders.length * 2 >= data.climbs.length ? boulders : data.climbs.filter((c) => !isBoulder(c));
		const scale = gradeScale(mine, data.scales);
		return scale && { scale, points: grades(mine, data.weeks, scale) };
	});
	const top = $derived.by(() => {
		const tops = main?.points.filter((t): t is number => t !== null) ?? [];
		return tops.length ? main!.scale.grades[Math.max(...tops)] : null;
	});
	const hoursLabel = (h: number) => `${Math.round(h * 10) / 10}h`;
</script>

<svelte:head><title>History</title></svelte:head>

{#snippet tile(label: string, value: string, note: string)}
	<span class="flex items-baseline justify-between gap-1">
		<span class="text-xs font-bold text-ink-2">{label}</span>
		<!-- Beside the list a tile has no room for "Hardest send" and a chevron. -->
		<span class="text-ink-3 lg:hidden"><Icon name="chevron-right" size="14px" stroke={2.6} /></span>
	</span>
	<span class="mt-1 digits text-[22px] leading-none">{value}</span>
	<span class="mt-0.5 text-xs font-semibold text-ink-2">{note}</span>
{/snippet}

<div class="lg:mx-auto lg:grid lg:max-w-[1280px] lg:grid-cols-[420px_minmax(0,1fr)]">
	<div class="flex flex-col gap-3.5 px-4 pt-[calc(env(safe-area-inset-top)+0.75rem)] pb-6 lg:px-6 lg:pt-3">
		<h1 class="page-title px-1">History</h1>

		{#if data.runs.length}
			<nav class="grid grid-cols-3 gap-2.5" aria-label="Progress">
				<a href="/history/progress?view=time" class="flex min-w-0 flex-col rounded-3xl bg-surface px-3.5 pt-3 pb-3.5 shadow-card">
					{@render tile('Time', hoursLabel(time.at(-1)!), 'this week')}
					<span class="mt-3.5"><Bars parts={[{ values: time, cls: 'bg-chart' }]} height={30} small /></span>
				</a>
				<a href="/history/progress?view=climbs" class="flex min-w-0 flex-col rounded-3xl bg-surface px-3.5 pt-3 pb-3.5 shadow-card">
					{@render tile('Climbs', String(climbs.at(-1)!.boulders + climbs.at(-1)!.routes), 'this week')}
					<span class="mt-3.5">
						<Bars
							parts={[
								{ values: climbs.map((c) => c.boulders), cls: 'bg-chart' },
								{ values: climbs.map((c) => c.routes), cls: 'bg-chart/50' }
							]}
							height={30}
							small
						/>
					</span>
				</a>
				<a href="/history/progress?view=grade" class="flex min-w-0 flex-col rounded-3xl bg-surface px-3.5 pt-3 pb-3.5 shadow-card">
					{@render tile('Hardest send', top ?? '–', '10 weeks')}
					<span class="mt-3.5">
						{#if main}<Dots
								n={main.points.length}
								marks={main.points.flatMap((v, i) => (v === null ? [] : [{ i, v, best: true }]))}
								label={(g) => main.scale.grades[g]}
								height={30}
								small
							/>{:else}<span class="block h-[30px]"></span>{/if}
					</span>
				</a>
			</nav>
		{/if}

		{#each months as m (m.label)}
			<section class="rounded-3xl bg-surface px-[18px] pt-4 pb-2 shadow-card">
				<h2 class="mb-1 text-[15px] font-bold">{m.label}</h2>
				<ul>
					{#each m.runs as r (r.id)}
						{@const picked = r.id === openId}
						<li class="list-row {picked ? 'picked' : ''}">
							<a
								href={wide.current ? `/history?open=${r.id}` : `/history/${r.id}`}
								data-sveltekit-replacestate={wide.current ? '' : undefined}
								data-sveltekit-noscroll={wide.current ? '' : undefined}
								aria-current={picked ? 'true' : undefined}
								class="flex min-h-16 items-center gap-3.5 py-2.5"
							>
								<span class="flex size-12 shrink-0 flex-col items-center justify-center rounded-2xl {picked ? 'bg-surface' : 'bg-well'}">
									<span class="text-xs leading-none font-bold tracking-wide text-ink-2 uppercase">{weekday(r.local_date)}</span>
									<span class="mt-0.5 digits text-xl leading-none">{Number(r.local_date.slice(8))}</span>
								</span>
								<span class="min-w-0 flex-1">
									<span class="block truncate text-[15px] font-bold">{r.name}</span>
									{#if facts(r)}<span class="mt-0.5 block truncate text-xs font-semibold text-ink-2">{facts(r)}</span>{/if}
								</span>
								<span class="text-ink-3 lg:hidden"><Icon name="chevron-right" size="18px" stroke={2.4} /></span>
							</a>
						</li>
					{/each}
				</ul>
			</section>
		{:else}
			<Empty title="No sessions yet" text="Finished sessions show here.">
				<Button href="/">Start one</Button>
			</Empty>
		{/each}
	</div>

	<!-- The pane scrolls on its own, so a long session leaves the list where it
	     was. It reaches into the tabs' bottom padding, which would otherwise
	     lift it at the end of the page. A link within it changes only the pane;
	     one to another page turns that off. It draws the divider, as it fills
	     the window and a short list does not. -->
	<div class="hidden lg:sticky lg:top-0 lg:-mb-8 lg:block lg:h-dvh lg:overflow-y-auto lg:border-l lg:border-line" bind:this={pane} data-sveltekit-replacestate data-sveltekit-noscroll>
		<div class="mx-auto flex max-w-[640px] flex-col gap-3.5 pb-8 {journaling ? 'px-4' : 'px-8 pt-3'}">
			<FormError message={paneError} />
			{#if exerciseId && exercise?.id === exerciseId}
				<div class="flex">
					<a
						href="/history?open={openId}"
						class="inline-flex h-11 items-center gap-0.5 rounded-full bg-surface pr-4 pl-2.5 text-[15px] font-bold text-ink shadow-card-sm"
					>
						<Icon name="chevron-left" size="1.25rem" stroke={2.4} />Session
					</a>
				</div>
				<header class="px-1">
					<h2 class="page-title text-balance">{exercise.name}</h2>
					<p class="mt-2 text-[15px] font-semibold text-ink-2">{plural(exercise.sessions.length, 'session')}</p>
				</header>
				<ExerciseHistory sessions={exercise.sessions} scales={exercise.scales} sides={exercise.perSide ? 2 : 1} href={(s) => `/history?open=${s.run}`} />
			{:else if journaling}
				{#if openRun.run?.id === openId}
					<SessionJournalForm
						cancel="/history?open={openId}"
						heading="h2"
						saved={async (r) => {
							opened = r;
							await goto(`/history?open=${r.id}`, { replaceState: true, noScroll: true });
						}}
					/>
				{/if}
			{:else if opened && opened.id === openId}
				{@const run = opened}
				<RunView
					{run}
					heading="h2"
					journalHref="/history?open={run.id}&journal=1"
					exerciseHref={(s) => `/history?open=${run.id}&exercise=${s.exercise}&name=${encodeURIComponent(s.name)}`}
				>
					{#snippet actions()}
						<a
							href="/run/{run.id}"
							data-sveltekit-replacestate="false"
							data-sveltekit-noscroll="false"
							class="flex h-11 shrink-0 items-center rounded-full bg-surface px-4 text-[15px] font-bold text-ink shadow-card-sm">Edit</a
						>
					{/snippet}
				</RunView>
			{/if}
		</div>
	</div>
</div>

<style>
	/* The open session's row, drawn as a well that reaches past the card's
	   dividers, as Journal's. */
	.picked {
		margin-inline: -0.625rem;
		padding-inline: 0.625rem;
		border-radius: 1rem;
		background: var(--well);
		box-shadow: none;
	}
	.picked + :global(li) {
		box-shadow: none;
	}
</style>
