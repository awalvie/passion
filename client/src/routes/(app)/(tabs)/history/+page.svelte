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
	import FormBar from '$lib/FormBar.svelte';
	import FormError from '$lib/FormError.svelte';
	import Icon from '$lib/Icon.svelte';
	import { counts, gradeScale, grades, hours, isBoulder, loaded, progressFrom, topSend, totalTries, type DatedClimb, type DatedSet } from '$lib/progress';
	import Tabs from '$lib/Tabs.svelte';
	import { urlFilters } from '$lib/filters.svelte';
	import { loadExerciseHistory, type Run, type RunSummary } from '$lib/run';
	import { openRun } from '$lib/runState.svelte';
	import RunView from '$lib/RunView.svelte';
	import SessionJournalForm from '$lib/SessionJournalForm.svelte';
	import { plural } from '$lib/text';

	let { data } = $props();

	// A kind is a run's template, or its name when it has none; it reads as its
	// newest run's name.
	const kindOf = (r: RunSummary) => r.template ?? r.name;
	const kinds = $derived.by(() => {
		const out = new Map<string, string>();
		for (const r of data.runs) if (!out.has(kindOf(r))) out.set(kindOf(r), r.name);
		return [...out].map(([value, label]) => ({ value, label }));
	});
	const filters = urlFilters(['kind'], true);
	// A kind with no sessions left, from an old link, shows them all.
	const kind = $derived(kinds.some((k) => k.value === filters.kind) ? filters.kind : '');
	const listed = $derived(kind ? data.runs.filter((r) => kindOf(r) === kind) : data.runs);

	// From lg a session opens in a pane beside the list, named by open; with
	// none named, or one the kind leaves out, the newest listed opens. An
	// exercise's name in it swaps the pane for that exercise's history, named
	// by exercise.
	const wide = new MediaQuery('min-width: 64rem');
	// journal swaps the pane for the session's journal form, which edits the
	// run through openRun as the form does in a session. Below lg the form
	// fills the window, so a narrowed window keeps what was typed.
	const journaling = $derived(page.url.searchParams.has('open') && page.url.searchParams.has('journal'));
	const asked = $derived(page.url.searchParams.get('open'));
	const openId = $derived(wide.current || journaling ? (listed.find((r) => r.id === asked) ?? listed[0])?.id : undefined);
	const exerciseId = $derived(wide.current ? page.url.searchParams.get('exercise') : null);
	const exerciseName = $derived(page.url.searchParams.get('name'));
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
	function fill<T>(fetch: () => Promise<T>, done: (v: T) => void, failed = () => {}) {
		let gone = false;
		paneError = '';
		fetch().then(
			(v) => !gone && done(v),
			(e) => {
				if (gone) return;
				paneError = describe(e, (x) => x);
				failed();
			}
		);
		return () => (gone = true);
	}
	$effect(() => {
		const id = openId;
		// The last session stays, dimmed, until the next comes, so with none
		// to come it goes.
		if (!id) opened = null;
		else
			return fill(
				() => request<Run>('GET', `/api/v1/runs/${id}`),
				(r) => (opened = r),
				() => (opened = null)
			);
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

	// edge sits where the strip pins, the notch's height above it.
	let edge = $state<HTMLElement>();
	let pinned = $state(false);
	$effect(() => {
		if (!edge) return;
		const o = new IntersectionObserver(([e]) => (pinned = !e.isIntersecting));
		o.observe(edge);
		return () => o.disconnect();
	});

	// Another kind picked deep in the list starts its list at the top, under
	// the strip.
	$effect(() => {
		void kind;
		untrack(() => pinned && edge && scrollTo({ top: edge.getBoundingClientRect().top + scrollY }));
	});

	let done = $state<{ climbs: DatedClimb[]; sets: DatedSet[] }>({ climbs: [], sets: [] });
	$effect(() => {
		let gone = false;
		// A failed load keeps what came before.
		data.done.then((d) => !gone && d && (done = d));
		return () => (gone = true);
	});
	const climbsOf = $derived(Map.groupBy(done.climbs, (c) => c.run));
	const setsOf = $derived(Map.groupBy(done.sets, (s) => s.run));
	// What a row says happened: the top send and the climbs, or else the sets.
	// A warm-up's sets say nothing.
	function outcome(r: RunSummary) {
		const cs = climbsOf.get(r.id) ?? [];
		const ss = (setsOf.get(r.id) ?? []).filter(loaded);
		const scale = gradeScale(cs, data.scales);
		const top = scale && topSend(cs, scale);
		const graded = cs.some((c) => c.grade_system);
		const join = (xs: unknown[]) => xs.filter(Boolean).join(' · ');
		const length = r.elapsed_seconds === null ? '' : formatMinutes(r.elapsed_seconds);
		const tries = cs.length && plural(totalTries(cs), 'climb');
		const head = join([top && `Top ${top.grade}`, graded && `${cs.filter((c) => c.sent).length} sent`, !cs.length && ss.length && plural(ss.length, 'set')]);
		// With one kind picked the name is the strip's, so what happened leads.
		// The place goes last, so a cut takes it first.
		const [title, line] = kind && head ? [head, join([length, tries])] : [r.name, join([length, head, tries])];
		return { title, line: join([line, r.place]) };
	}

	const months = $derived.by(() => {
		const out: { label: string; runs: RunSummary[] }[] = [];
		for (const r of listed) {
			const label = formatDate(r.local_date, { month: 'long', year: 'numeric' });
			if (out.at(-1)?.label !== label) out.push({ label, runs: [] });
			out.at(-1)!.runs.push(r);
		}
		return out;
	});

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
		<!-- Beside the list a tile has no room for its label and a chevron. -->
		<span class="text-ink-3 lg:hidden"><Icon name="chevron-right" size="14px" stroke={2.6} /></span>
	</span>
	<span class="mt-1 digits text-[22px] leading-none">{value}</span>
	<span class="mt-0.5 text-xs font-semibold text-ink-2">{note}</span>
{/snippet}

<div class="lg:mx-auto lg:grid lg:max-w-[1280px] lg:grid-cols-[420px_minmax(0,1fr)]">
	<div class="{journaling ? 'hidden lg:flex' : 'flex'} flex-col gap-3.5 px-4 pt-[calc(env(safe-area-inset-top)+0.75rem)] pb-6 lg:px-6 lg:pt-3">
		<h1 class="page-title px-1">History</h1>

		{#if data.runs.length}
			<nav class="grid grid-cols-3 gap-2.5" aria-label="Progress">
				<a href="/history/progress?view=time" class="flex min-w-0 flex-col rounded-3xl bg-surface px-3.5 pt-3 pb-3.5 shadow-card">
					{@render tile('Time', hoursLabel(time.at(-1)!), 'this week')}
					<span class="mt-auto pt-3.5"><Bars parts={[{ values: time, cls: 'bg-chart' }]} height={30} small /></span>
				</a>
				<a href="/history/progress?view=climbs" class="flex min-w-0 flex-col rounded-3xl bg-surface px-3.5 pt-3 pb-3.5 shadow-card">
					{@render tile('Climbs', String(climbs.at(-1)!.boulders + climbs.at(-1)!.routes), 'this week')}
					<span class="mt-auto pt-3.5">
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
					{@render tile('Top send', top ?? '–', '10 weeks')}
					<span class="mt-auto pt-3.5">
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

			<!-- The strip stays in reach as the list scrolls under it. Its backing
			     shows only once it is pinned, or it would cut the tiles' shadows. -->
			<div bind:this={edge} class="relative top-[calc(-1*env(safe-area-inset-top))] -mb-3.5 h-0"></div>
			<div class="sticky top-[env(safe-area-inset-top)] z-20 -mx-4 px-4 lg:-mx-6 lg:px-6 {pinned ? 'bg-ground/95 backdrop-blur-md' : ''}">
				<Tabs label="Session" bind:value={() => kind, (v) => (filters.kind = v)} options={[{ value: '', label: 'All' }, ...kinds]} />
			</div>
		{/if}

		{#each months as m (m.label)}
			<section class="rounded-3xl bg-surface px-[18px] pt-4 pb-2 shadow-card">
				<h2 class="mb-1 text-[15px] font-bold">{m.label}</h2>
				<ul>
					{#each m.runs as r (r.id)}
						{@const o = outcome(r)}
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
									<span class="block truncate text-[15px] font-bold">{o.title}</span>
									{#if o.line}<span class="mt-0.5 block truncate text-xs font-semibold text-ink-2">{o.line}</span>{/if}
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
	<div class="{journaling ? '' : 'hidden'} lg:sticky lg:top-0 lg:-mb-8 lg:block lg:h-dvh lg:overflow-y-auto lg:border-l lg:border-line" bind:this={pane} data-sveltekit-replacestate data-sveltekit-noscroll>
		<div class="mx-auto flex max-w-[640px] flex-col gap-3.5 pb-8 {journaling ? 'lg:px-4' : 'px-8 pt-3'}">
			<FormError message={journaling ? '' : paneError} />
			{#if exerciseId && exercise?.id === exerciseId}
				{@const progress = progressFrom(page.url.searchParams.get('from'))}
				<div class="flex">
					<a
						href={progress ?? `/history?open=${openId}`}
						data-sveltekit-replacestate={progress ? 'false' : undefined}
						data-sveltekit-noscroll={progress ? 'false' : undefined}
						class="inline-flex h-11 items-center gap-0.5 rounded-full bg-surface pr-4 pl-2.5 text-[15px] font-bold text-ink shadow-card-sm"
					>
						<Icon name="chevron-left" size="1.25rem" stroke={2.4} />{progress ? 'Progress' : 'Session'}
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
				{:else}
					<!-- While the run loads, or when it fails to, the bar keeps a way back. -->
					<FormBar title="Session journal" cancel="/history?open={openId}" heading="h2" disabled error={paneError} />
				{/if}
			{:else if opened}
				<!-- While the next session loads, the last one stays, dimmed. A
				     quick load comes before the dimming. -->
				{@const run = opened}
				{@const stale = run.id !== openId}
				<div class="transition-opacity {stale ? 'opacity-40 delay-150' : ''}" aria-busy={stale} inert={stale}>
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
				</div>
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
