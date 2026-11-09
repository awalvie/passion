<script lang="ts">
	import { goto, invalidateAll } from '$app/navigation';
	import { page } from '$app/state';
	import { describe } from '$lib/api';
	import Button from '$lib/Button.svelte';
	import { addDays, dayDate, daysBetween, formatDate, fullDate } from '$lib/dates';
	import { formatDuration } from '$lib/exercise';
	import FormError from '$lib/FormError.svelte';
	import GoalCards from '$lib/GoalCards.svelte';
	import HeroCard from '$lib/HeroCard.svelte';
	import Icon from '$lib/Icon.svelte';
	import Empty from '$lib/Empty.svelte';
	import Menu from '$lib/Menu.svelte';
	import { DayActions } from '$lib/dayActions.svelte';
	import DaySheets from '$lib/DaySheets.svelte';
	import type { ScheduledDay } from '$lib/plan';
	import { currentStop, stepsOf, type Run, type RunSummary } from '$lib/run';
		import Info from '$lib/Info.svelte';
	import { discardRun, openRun, startRun, type StartBody } from '$lib/runState.svelte';
	import SessionIcon, { iconState } from '$lib/SessionIcon.svelte';
	import { plural } from '$lib/text';
	import { request } from '$lib/api';
	import { trends, trendWeeks, type DatedClimb } from '$lib/progress';
	import Topo from '$lib/Topo.svelte';
	import TrendTile from '$lib/TrendTile.svelte';
	import { topTopo } from '$lib/topo';
	import WeekStrip from '$lib/WeekStrip.svelte';
	import { MediaQuery } from 'svelte/reactivity';

	let { data } = $props();
	const day = new DayActions();

	// From lg the journal opens in Journal's pane rather than on its own page.
	const wide = new MediaQuery('min-width: 64rem');

	// The day or session being started, or 'open'.
	let starting = $state<string | null>(null);
	let error = $state('');

	async function start(body: StartBody) {
		starting = 'scheduled' in body ? body.scheduled : 'template' in body ? body.template : 'open';
		error = '';
		try {
			const run = await startRun(body);
			await goto(`/run/${run.id}`);
		} catch (e) {
			error = describe(e, (f) => f);
			// The run can start even when the answer is lost, and then the card
			// offers to go back to it.
			await invalidateAll();
		} finally {
			starting = null;
		}
	}

	async function discard(r: RunSummary) {
		error = '';
		try {
			if (await discardRun(r)) await invalidateAll();
		} catch (e) {
			error = describe(e, (f) => f);
		}
	}

	// Runs come from the layout, which can lose the signal on its own.
	const offline = $derived(data.offline || data.runs === null);

	const sunday = $derived(addDays(data.monday, 6));
	const finished = $derived((data.runs ?? []).filter((r) => r.finished_at !== null));
	const thisWeek = (r: RunSummary) => data.monday <= r.local_date && r.local_date <= sunday;

	const span = $derived(Array.from({ length: 7 }, (_, i) => addDays(data.monday, i)));
	// A tap on the week strip shows that day in place of today.
	const shown = $derived.by(() => {
		const day = page.url.searchParams.get('day');
		return span.includes(day ?? '') ? day! : data.today;
	});
	const other = $derived(shown !== data.today);
	const shownDays = $derived(data.days.filter((d) => d.local_date === shown));

	const live = $derived(data.live && openRun.run?.id === data.live.id ? openRun.run : null);
	// A day whose run was left open shows as that run's Not finished card.
	const planned = $derived(
		shownDays.filter((d) => d.status !== 'done' && !(live && d.run === live.id) && !data.stale.some((r) => r.id === d.run))
	);
	// Today lists every run left open; another day only its own.
	const stale = $derived(
		other ? data.stale.filter((r) => r.local_date === shown || shownDays.some((d) => d.run === r.id)) : data.stale
	);
	// A run belongs to the day it was done, and to the planned day it ran for.
	const done = $derived(finished.filter((r) => r.local_date === shown || shownDays.some((d) => d.run === r.id)));

	const date = $derived(fullDate(shown));

	function away(n: number) {
		if (n === 1) return 'tomorrow';
		if (n === -1) return 'yesterday';
		return n > 0 ? `in ${n} days` : `${-n} days ago`;
	}

	function when(date: string) {
		const n = daysBetween(data.today, date);
		return n === 1 ? 'Tomorrow' : n < 7 ? formatDate(date, { weekday: 'long' }) : dayDate(date);
	}

	const ranThisWeek = $derived(finished.filter(thisWeek));

	// How long a session usually takes, to five minutes; more exact than that it
	// cannot be.
	function usualOf(d: ScheduledDay) {
		const timed = finished.filter((r) => r.local_date < data.today && r.template === d.template && r.elapsed_seconds);
		if (!timed.length) return null;
		const mean = timed.reduce((s, r) => s + r.elapsed_seconds!, 0) / timed.length;
		return formatDuration(Math.max(1, Math.round(mean / 300)) * 300);
	}

	const sessionsById = $derived(new Map(data.library.map((t) => [t.id, t])));

	const stop = $derived(live ? currentStop(live) : null);

	// The first-week card, for someone who has logged nothing yet.
	const setup = $derived(
		!other && data.runs?.length === 0 && data.centres !== null
			? [
					{ text: 'Add where you train', href: '/profile', icon: 'map-pin', done: data.centres > 0 },
					{ text: 'Plan your week', href: '/plan', icon: 'calendar', done: data.week.length > 0 || data.cycles.length > 0 }
				] as const
			: null
	);
	// Hidden for good on this phone; the server keeps no flag for it.
	let setupHidden = $state(
		(() => {
			try {
				return localStorage.getItem('passion-hide-setup') === '1';
			} catch {
				return false;
			}
		})()
	);
	function hideSetup() {
		setupHidden = true;
		try {
			localStorage.setItem('passion-hide-setup', '1');
		} catch {
			/* hidden until the app reloads */
		}
	}
	const settingUp = $derived(!setupHidden && (setup?.some((s) => !s.done) ?? false));

	// The tiles of Progress, for the room a laptop has. A phone shows none, so it
	// loads none.
	let recent = $state<DatedClimb[] | null>(null);
	const weeks = $derived(trendWeeks(data.today));
	$effect(() => {
		if (!wide.current || offline) return;
		request<{ climbs: DatedClimb[] }>('GET', `/api/v1/climbs?from=${weeks[0].from}&to=${weeks[7].to}`).then(
			(r) => (recent = r.climbs),
			() => (recent = null)
		);
	});
	const tiles = $derived.by(() => {
		const ran = finished.filter((r) => r.local_date >= weeks[0].from && r.local_date <= weeks[7].to);
		// Nothing in the last four weeks leaves no figure to show.
		if (!recent || !ran.some((r) => r.local_date >= weeks[4].from)) return [];
		const first = finished.reduce((d, r) => (r.local_date < d ? r.local_date : d), data.today);
		const t = trends(ran, recent, weeks, first);
		return [
			{ label: 'Hours a week', t: t.hours, unit: 'h', view: 'time' },
			{ label: 'Climbs a week', t: t.climbs, unit: '', view: 'climbs' },
			{ label: 'Send rate', t: t.sent, unit: '%', view: 'climbs' },
			{ label: 'Flash rate', t: t.flash, unit: '%', view: 'climbs' }
		];
	});
	// The weights of the shown day's first session last time, as its own page
	// shows them. With nothing left to do today, a laptop shows the next session.
	const ahead = $derived(!other && !live && !offline && !planned.length ? data.next : null);
	const prepDay = $derived(planned.find((d) => d.status !== 'missed') ?? ahead);
	let lastRun = $state<Run | null>(null);
	$effect(() => {
		lastRun = null;
		if (!wide.current) return;
		const id = prepDay?.template;
		const past = finished.filter((r) => r.template === id).sort((a, b) => b.started_at.localeCompare(a.started_at));
		// The card shows fine without them.
		if (past[0]) request<Run>('GET', `/api/v1/runs/${past[0].id}`).then((r) => (lastRun = r), () => {});
	});
	const prep = $derived.by(() => {
		if (!prepDay || live) return null;
		const run = lastRun && lastRun.template === prepDay.template ? lastRun : null;
		const names = new Map(run ? stepsOf(run).map((x) => [x.exercise, x.name]) : []);
		const loads = run
			? [...new Set(stepsOf(run).map((x) => x.exercise))].filter((e) => run.sets.some((x) => x.exercise === e)).flatMap((exercise) => {
					const sets = run.sets.filter((x) => x.exercise === exercise);
					const top = sets.reduce((a, b) => ((b.weight_kg ?? -Infinity) > (a.weight_kg ?? -Infinity) ? b : a));
					if (!top.weight_kg) return [];
					const dose = top.reps ? `${sets.length}×${top.reps}` : plural(sets.length, 'set');
					return [{ exercise, name: names.get(exercise) ?? 'Exercise', text: `${dose} · ${top.weight_kg} kg` }];
				})
			: [];
		return { loads, last: run?.id };
	});
</script>

<svelte:head><title>Today</title></svelte:head>

<div
	class="pointer-events-none absolute inset-x-0 top-0 -z-0 h-[420px] bg-[radial-gradient(120%_70%_at_100%_-10%,var(--glow-1),transparent_62%),radial-gradient(90%_60%_at_-10%_18%,var(--glow-2),transparent_62%)]"
></div>
<Topo
	shape={topTopo}
	class="absolute inset-x-0 top-0 h-80 w-full text-[var(--topo)] [mask-image:linear-gradient(#000_30%,transparent)]"
/>

<div class="relative pt-[env(safe-area-inset-top)] lg:mx-auto lg:max-w-[1200px] lg:px-4 lg:pb-8">
	<div class="flex flex-col">
		<header class="flex items-start justify-between gap-3 px-4 pt-3">
			<div class="px-1">
				<h1 class="page-title">
					{other ? formatDate(shown, { weekday: 'long' }) : 'Today'}
				</h1>
				<p class="mt-1 text-[15px] font-semibold text-ink-2">
					{other ? `${formatDate(shown, { day: 'numeric', month: 'long' })} · ${away(daysBetween(data.today, shown))}` : date}
				</p>
			</div>
			<div class="flex min-w-0 items-center gap-2">
				{#if other}
					<a
						href="/"
						class="flex h-11 shrink-0 items-center gap-1.5 rounded-full bg-surface px-4 text-[15px] font-bold text-ink shadow-card-sm"
						data-sveltekit-replacestate
						data-sveltekit-noscroll
					>
						<Icon name="undo-2" size="1.125rem" />Today
					</a>
				{/if}
				<a href="/profile" class="flex size-11 shrink-0 items-center justify-center rounded-full bg-surface text-ink shadow-card-sm md:hidden" aria-label="Profile">
					<Icon name="settings" size="1.25rem" />
				</a>
			</div>
		</header>

		<div class="px-4"><FormError message={error} /></div>

		<section class="mx-4 mt-3.5 rounded-3xl bg-surface pt-1 pb-4 shadow-card lg:hidden">
			{@render strip()}
		</section>

		<div class="contents lg:grid lg:grid-cols-[minmax(0,1fr)_392px] lg:items-start">
		<div class="contents lg:flex lg:flex-col">

		{#if settingUp}
			<HeroCard>
				<button
					type="button"
					class="absolute top-3 right-3 flex size-11 items-center justify-center rounded-full bg-white/10 text-on-hero"
					aria-label="Hide the setup card"
					onclick={hideSetup}
				>
					<Icon name="x" size="1.125rem" stroke={2.4} />
				</button>
				<p class="text-xs font-semibold tracking-[0.06em] text-on-hero-2 uppercase">Get set up</p>
				<h2 class="mt-1 pr-10 text-[32px] leading-tight font-extrabold tracking-tight">Your first week</h2>
				<ul class="mt-2 flex flex-col">
					{#each setup as s (s.text)}
						<li>
							{#if s.done}
								<p class="flex min-h-12 items-center gap-3 text-[17px] font-bold">
									<span class="flex size-8 shrink-0 items-center justify-center rounded-full bg-tint text-on-tint">
										<Icon name="check" size="1rem" stroke={2.6} />
									</span>
									{s.text}
								</p>
							{:else}
								<a href={s.href} class="flex min-h-12 items-center gap-3 text-[17px] font-bold">
									<span class="flex size-8 shrink-0 items-center justify-center rounded-full shadow-[inset_0_0_0_2px_rgba(242,246,234,0.35)]">
										<Icon name={s.icon} size="1rem" stroke={2.4} />
									</span>
									<span class="flex-1">{s.text}</span>
									<Icon name="chevron-right" size="18px" stroke={2.2} />
								</a>
							{/if}
						</li>
					{/each}
				</ul>
			</HeroCard>
		{/if}

		<div class="contents">
		{#if live && !other}
			{@render liveSlot(live)}
		{:else}
			{#each planned as d, i (d.id)}
				{@render sessionSlot(d, !other && i === 0)}
			{/each}
			{#if done.length}
				{#each done as r, i (r.id)}
					<HeroCard>
						<div class="relative">
							<a href="/history/{r.id}" class="absolute -inset-[18px] rounded-[inherit]" aria-label="Open {r.name}"></a>
							<div class="flex items-center gap-3">
								<span class="flex size-11 shrink-0 items-center justify-center rounded-full bg-on-hero text-[var(--hero)]"><Icon name="check" size="1.25rem" stroke={2.6} /></span>
								<h2 class="min-w-0 flex-1 truncate text-xl font-bold tracking-tight">{r.name}</h2>
								{#if r.has_journal === false}<a href="/run/{r.id}/finish" class="relative hidden h-12 items-center justify-center gap-2 rounded-full bg-white/10 px-5 text-[15px] font-semibold text-on-hero lg:flex"><Icon name="pencil" size="1.125rem" stroke={2.2} />Add session journal</a>{/if}
							</div>
							{#if r.has_journal === false}<a href="/run/{r.id}/finish" class="relative mt-4 flex h-12 items-center justify-center gap-2 rounded-full bg-white/10 text-[15px] font-semibold text-on-hero lg:hidden"><Icon name="pencil" size="1.125rem" stroke={2.2} />Add session journal</a>{/if}
						</div>
					</HeroCard>
				{/each}
			{:else if other && !planned.length}
				{#if !stale.length}
					<section class="mx-4 mt-3.5 rounded-3xl bg-surface p-[18px] shadow-card">
						<p class="text-[28px] leading-tight font-extrabold tracking-tight">Nothing on this day</p>
					</section>
				{/if}
			{:else if !planned.length}
				{#if offline}
					<section class="mx-4 mt-3.5 rounded-3xl bg-surface p-[18px] shadow-card">
						<p class="text-[28px] leading-tight font-extrabold tracking-tight">No signal</p>
						<p class="mt-1 text-[15px] font-semibold text-ink-2">The plan cannot load. A running session still works.</p>
					</section>
				{:else if data.next}
					<HeroCard>
						<div class="relative">
							<div class="flex items-center gap-3">
								<span class="flex size-11 shrink-0 items-center justify-center rounded-full bg-white/10 text-on-hero"><Icon name="moon" size="1.25rem" /></span>
								<h2 class="text-xl font-bold tracking-tight">Rest day</h2>
							</div>
						</div>
					</HeroCard>
				{:else if !settingUp}
					<div class="mx-4 mt-3.5">
						<Empty title="Nothing planned" text="Plan your week, or log a session as you go."><div class="lg:w-60"><Button href="/plan">Plan your week</Button></div></Empty>
					</div>
				{/if}
			{/if}
		{/if}

		</div>

		{#if stale.length}
			<ul class="mx-4 mt-3.5 rounded-3xl bg-surface px-[18px] py-1 shadow-card">
				{#each stale as r (r.id)}
					<li class="list-row flex items-center gap-2">
						<a href="/run/{r.id}/finish" class="flex min-h-[68px] min-w-0 flex-1 items-center gap-3">
							<span class="flex size-11 shrink-0 items-center justify-center rounded-full text-ink-2 outline-[1.5px] -outline-offset-[1.5px] outline-dashed outline-ink-3"><Icon name="pause" size="1.125rem" /></span>
							<span class="min-w-0 flex-1">
								<span class="block truncate text-[15px] font-bold">{r.name}</span>
								<span class="block text-xs font-semibold text-ink-2">Not finished · {formatDate(r.local_date, { weekday: 'long' })}</span>
							</span>
						</a>
						<Menu look="bg-well text-ink" items={[{ label: 'Finish', onclick: () => goto(`/run/${r.id}/finish`) }, { label: 'Discard session', danger: true, onclick: () => discard(r) }]} />
					</li>
				{/each}
			</ul>
		{/if}

		{#if !offline && !other}{@render actions('lg:hidden')}{/if}
		{@render prepare()}

		{#if tiles.length && !offline}
			<section class="mx-4 mt-6 hidden lg:block">
				<div class="relative flex items-center justify-between px-1">
					<a href="/history/progress?view=time&range=4w&from=/" class="flex h-11 items-center gap-1 text-xl font-bold tracking-tight">Last 4 weeks<Icon name="chevron-right" size="1.125rem" stroke={2.4} /></a>
					<Info text="Each figure is a week on average over the last 4 weeks, and its change is on the 4 weeks before. The bars are the 8 weeks: the 4 before pale, the last 4 green." />
				</div>
				<ul class="mt-1 grid grid-cols-4 gap-3.5">
					{#each tiles.filter((t) => t.t.now !== null) as t (t.label)}<TrendTile label={t.label} t={t.t} unit={t.unit} href="/history/progress?view={t.view}&range=4w&from=/" />{/each}
				</ul>
			</section>
		{/if}
		</div>
		<div class="contents lg:flex lg:flex-col">
		<section class="mx-4 mt-3.5 hidden rounded-3xl bg-surface pt-1 pb-4 shadow-card lg:block">
			{@render strip()}
		</section>
		{#if !offline && !other}{@render actions('max-lg:hidden')}{/if}
		{#if !offline}
			<GoalCards goals={data.goals} today={data.today} />
		{/if}

		{#if data.pinned.length}
			<a href="/journal" class="mx-4 mt-3.5 hidden h-11 items-center gap-1 px-1 text-[15px] font-bold lg:flex">Pinned<span class="text-ink-3"><Icon name="chevron-right" size="1rem" /></span></a>
			<section class="mx-4 mt-3.5 lg:mt-0 rounded-3xl bg-surface px-[18px] pt-4 shadow-card lg:pt-1 {data.pinned.length > 2 ? 'pb-1' : 'pb-2'}">
				<div class="flex items-baseline justify-between lg:hidden">
					<h2 class="text-[15px] font-bold">Pinned</h2>
					<a href="/journal" class="-my-3 flex h-11 items-center gap-0.5 text-xs font-bold text-ink">Journal<Icon name="chevron-right" size="14px" /></a>
				</div>
				<ul class="mt-1 lg:mt-0">
					{#each data.pinned.slice(0, 2) as e (e.id)}
						<li class="list-row">
							<a href="/journal/{e.id}?from=/" class="block py-3 text-[15px] leading-[1.55] font-medium">
								<span class="line-clamp-6">{e.body}</span>
							</a>
						</li>
					{/each}
					{#if data.pinned.length > 2}
						<li class="list-row">
							<a href="/journal" class="flex min-h-12 items-center justify-between text-[15px] font-bold text-ink-2">
								{data.pinned.length - 2} more pinned<Icon name="chevron-right" size="18px" stroke={2.4} />
							</a>
						</li>
					{/if}
				</ul>
			</section>
		{/if}
		</div>
		</div>

	</div>
</div>

{#snippet startButton(d: ScheduledDay)}
	<Button disabled={starting !== null} onclick={() => start({ scheduled: d.id })}>{starting === d.id ? 'Starting…' : 'Start'}</Button>
{/snippet}

{#snippet usual(d: ScheduledDay, faint: string)}
	{@const u = usualOf(d)}
	{#if u}<span class="hidden text-xs font-semibold lg:block {faint}">Usually {u}</span>{/if}
{/snippet}

{#snippet prepare()}
	{#if ahead}
		<section class="mx-4 mt-6 hidden lg:block">
			<h2 class="px-1 text-xl font-bold tracking-tight">Next</h2>
			<div class="relative mt-2 rounded-3xl bg-surface p-[18px] shadow-card">
				<a href="/templates/{ahead.template}?from=/" class="flex items-center gap-3">
					<SessionIcon icon={ahead.template_icon} name={ahead.template_name} size={44} />
					<span class="min-w-0 flex-1">
						<span class="block truncate text-xl font-bold tracking-tight">{ahead.template_name}</span>
						<span class="block text-xs font-semibold text-ink-2">{when(ahead.local_date)}{usualOf(ahead) ? ` · usually ${usualOf(ahead)}` : ''}</span>
					</span>
					<span class="text-ink-3"><Icon name="chevron-right" size="18px" stroke={2.4} /></span>
				</a>
				{@render panels(ahead, 'bg-well', 'border-line', 'text-ink-2')}
			</div>
		</section>
	{/if}
{/snippet}

{#snippet panels(d: ScheduledDay, well: string, line: string, faint: string)}
	{@const loads = prep?.loads.length && prepDay?.id === d.id ? prep.loads.slice(0, 6) : []}
	{#if loads.length}
		<a href="/history/{prep!.last}?from=/" class="relative mt-4 hidden rounded-2xl px-3.5 py-1 lg:block {well}">
			<span class="block pt-2 pb-1 text-xs font-semibold {faint}">Last time</span>
			<span class="grid grid-cols-2 gap-x-8">
				{#each loads as l, i (l.exercise)}
					<span class="flex min-w-0 items-baseline gap-3 py-2.5 {i > 1 ? `border-t ${line}` : ''}">
						<span class="min-w-0 flex-1 truncate text-[15px] font-semibold">{l.name}</span>
						<span class="digits shrink-0 text-[15px]">{l.text}</span>
					</span>
				{/each}
			</span>
		</a>
	{/if}
{/snippet}

{#snippet actions(cls: string)}
	<div class="{cls} mx-4 mt-3.5 flex rounded-3xl bg-surface shadow-card">
		{#if !live}<button type="button" disabled={starting !== null} onclick={() => start({ name: 'Open session' })} class="flex h-14 flex-1 items-center justify-center gap-2 text-[15px] font-semibold disabled:opacity-50"><Icon name="plus" size="1.125rem" stroke={2.2} />{starting === 'open' ? 'Starting…' : 'Open session'}</button><span class="my-3 w-px bg-line"></span>{/if}
		<a href={wide.current ? '/journal?new=1' : '/journal/new?from=/'} class="flex h-14 flex-1 items-center justify-center gap-2 text-[15px] font-semibold"><Icon name="pencil" size="1.125rem" stroke={2.2} />Journal entry</a>
	</div>
{/snippet}

{#snippet strip()}
	<WeekStrip monday={data.monday} today={data.today} week={data.week} selected={shown} ran={ranThisWeek.map((r) => r.local_date)} />
{/snippet}

{#snippet sessionSlot(d: ScheduledDay, primary: boolean)}
	{#if d.status === 'missed' || other}
	<section class="mx-4 mt-3.5 rounded-3xl bg-surface p-[18px] shadow-card">
		<div class="flex items-center gap-3">
			<a href="/templates/{d.template}?from=/" class="flex min-w-0 flex-1 items-center gap-3">
				<SessionIcon icon={d.template_icon} name={d.template_name} state={iconState(d, data.today)} size={44} />
				<span class="min-w-0 flex-1">
					<span class="block truncate text-xl font-bold tracking-tight {d.status === 'missed' ? 'text-ink-2' : ''}">{d.template_name}</span>
					{#if d.status !== 'missed'}{@render usual(d, 'text-ink-2')}{/if}
				</span>
			</a>
			<Menu look="bg-well text-ink" items={[{ label: 'Move to another day', onclick: () => day.openMove(d) }, { label: 'Remove', danger: true, onclick: () => day.remove(d) }]} />
		</div>
		{#if d.status !== 'missed'}{@render panels(d, 'bg-well', 'border-line', 'text-ink-2')}{/if}
	</section>
	{:else}
	<HeroCard>
		<div class="relative">
			<a href="/templates/{d.template}?from=/" class="absolute -inset-[18px] rounded-[inherit]" aria-label="Open {d.template_name}"></a>
			<div class="flex items-center gap-3">
				<SessionIcon icon={d.template_icon} name={d.template_name} size={44} />
				<div class="min-w-0 flex-1"><h2 class="text-xl leading-tight font-bold tracking-tight">{d.template_name}</h2>{#if primary}{@render usual(d, 'text-on-hero-2')}{/if}</div>
				{#if !other}<div class="relative hidden w-44 lg:block">{@render startButton(d)}</div>{/if}
				<div class="relative"><Menu look="bg-white/10 text-on-hero" items={[{ label: 'Move to another day', onclick: () => day.openMove(d) }, { label: 'Remove', danger: true, onclick: () => day.remove(d) }]} /></div>
			</div>
			{#if primary}{@render panels(d, 'bg-white/10 backdrop-blur-md', 'border-white/10', 'text-on-hero-2')}{/if}
			{#if !other}<div class="relative mt-4 lg:hidden">{@render startButton(d)}</div>{/if}
		</div>
	</HeroCard>
	{/if}
{/snippet}

{#snippet liveSlot(run: Run)}
	<HeroCard>
		<div class="flex items-center gap-3">
			<a href="/run/{run.id}" class="flex min-w-0 flex-1 items-center gap-3" aria-label="Back to {run.name}">
				<SessionIcon icon={sessionsById.get(run.template ?? '')?.icon ?? null} name={run.name} size={44} />
				<span class="min-w-0">
					<span class="flex items-center gap-3"><span class="min-w-0 truncate text-xl font-bold tracking-tight">{run.name}</span><i class="live-dot shrink-0"></i></span>
					{#if stop}<span class="hidden text-xs font-semibold text-on-hero-2 lg:block">Now: {stop.step?.name ?? stop.choice?.name}</span>{/if}
				</span>
			</a>
			<div class="hidden w-44 lg:block"><Button href="/run/{run.id}">Continue</Button></div>
		</div>
	</HeroCard>
{/snippet}

<DaySheets {day} repeats={[]} />
