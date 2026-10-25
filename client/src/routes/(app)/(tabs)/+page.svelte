<script lang="ts">
	import { goto, invalidateAll } from '$app/navigation';
	import { page } from '$app/state';
	import { describe } from '$lib/api';
	import Bars from '$lib/Bars.svelte';
	import Button from '$lib/Button.svelte';
	import { addDays, cycleWeek, dayDate, daysBetween, formatDate, fullDate, mondayOf, shortDate } from '$lib/dates';
	import { formatDuration, formatMinutes } from '$lib/exercise';
	import FormError from '$lib/FormError.svelte';
	import HeroCard from '$lib/HeroCard.svelte';
	import Icon from '$lib/Icon.svelte';
	import Menu from '$lib/Menu.svelte';
	import type { ScheduledDay } from '$lib/plan';
	import { buckets, hours, tenWeeks } from '$lib/progress';
	import { currentStop, secondsSince, type Run, type RunSummary } from '$lib/run';
	import { discardRun, openRun, startRun, type StartBody } from '$lib/runState.svelte';
	import SessionIcon, { iconState } from '$lib/SessionIcon.svelte';
	import { plural } from '$lib/text';
	import Topo from '$lib/Topo.svelte';
	import { topTopo } from '$lib/topo';
	import WeekStrip from '$lib/WeekStrip.svelte';

	let { data } = $props();

	// The day being started, or 'open'.
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

	let now = $state(Date.now());
	$effect(() => {
		if (!live) return;
		const tick = setInterval(() => (now = Date.now()), 1000);
		return () => clearInterval(tick);
	});
	// Runs come from the layout, which can lose the signal on its own.
	const offline = $derived(data.offline || data.runs === null);

	const sunday = $derived(addDays(data.monday, 6));
	const finished = $derived((data.runs ?? []).filter((r) => r.finished_at !== null));
	const thisWeek = (r: RunSummary) => data.monday <= r.local_date && r.local_date <= sunday;

	// A tap on the week strip shows that day in place of today.
	const shown = $derived.by(() => {
		const day = page.url.searchParams.get('day');
		return Array.from({ length: 7 }, (_, i) => addDays(data.monday, i)).includes(day ?? '') ? day! : data.today;
	});
	const other = $derived(shown !== data.today);
	const shownDays = $derived(data.week.filter((d) => d.local_date === shown));

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

	const cycle = $derived(data.cycles.find((c) => c.starts <= data.today && data.today <= c.ends));
	const progress = $derived(cycle ? cycleWeek(cycle.starts, cycle.ends, data.today) : null);

	const weekPlanned = $derived(data.week.length);
	const weekDone = $derived(data.week.filter((d) => d.status === 'done').length);
	const missed = $derived(
		data.week.filter((d) => d.status === 'missed').sort((a, b) => a.local_date.localeCompare(b.local_date))
	);
	const ranThisWeek = $derived(finished.filter(thisWeek));
	const extra = $derived(ranThisWeek.filter((r) => !data.week.some((d) => d.run === r.id)).length);
	const ranLastWeek = $derived(
		finished.filter((r) => addDays(data.monday, -7) <= r.local_date && r.local_date < data.monday).length
	);

	// The usual length is to five minutes; more exact than that it cannot be.
	function factsOf(d: ScheduledDay) {
		const past = finished.filter((r) => r.local_date < data.today && r.template === d.template);
		const timed = past.filter((r) => r.elapsed_seconds);
		const mean = timed.reduce((s, r) => s + r.elapsed_seconds!, 0) / timed.length;
		const last = past.map((r) => r.local_date).sort().at(-1);
		return {
			usual: timed.length ? formatDuration(Math.max(1, Math.round(mean / 300)) * 300) : null,
			since: last ? daysBetween(last, data.today) : null
		};
	}

	// Your own sessions, and the built-in ones you have run, the last done
	// first. finished is newest first, so a template's first run is its last.
	const lastDone = $derived(new Map(finished.toReversed().map((r) => [r.template, r.local_date])));
	const mine = $derived(
		data.library
			.filter((t) => !t.shipped || lastDone.has(t.id))
			.sort((a, b) => (lastDone.get(b.id) ?? '').localeCompare(lastDone.get(a.id) ?? ''))
	);
	// A session planned for today starts as that plan, so the day counts as done.
	function startMine(template: string) {
		const d = data.week.find((d) => d.local_date === data.today && d.status === 'planned' && d.template === template);
		start(d ? { scheduled: d.id } : { template });
	}
	function doneWhen(template: string) {
		const last = lastDone.get(template);
		if (!last) return 'Not done yet';
		const n = daysBetween(last, data.today);
		return n ? `Last done ${away(-n)}` : 'Done today';
	}

	// The days a wide screen lists beside Today, last week to the end of next.
	const span = $derived(Array.from({ length: 21 }, (_, i) => addDays(data.monday, i - 7)));
	// Each listed day with something on it, and today.
	type Entry = { run: RunSummary; plan?: never } | { plan: ScheduledDay; run?: never };
	// A run shows on the planned day it ran for, else on the day it was done.
	const listed = $derived.by(() => {
		const runs = new Map((data.runs ?? []).map((r) => [r.id, r]));
		const claimed = new Set(data.days.map((d) => d.run));
		const out: { monday: string; dates: { date: string; entries: Entry[] }[] }[] = [];
		for (const date of span) {
			const entries: Entry[] = [
				...data.days
					.filter((d) => d.local_date === date)
					.map((d): Entry | null => (d.run ? (runs.has(d.run) ? { run: runs.get(d.run)! } : null) : { plan: d }))
					.filter((e) => e !== null),
				...(data.runs ?? [])
					.filter((r) => r.local_date === date && !claimed.has(r.id))
					.map((run) => ({ run }))
			].sort((a, b) => (a.run?.started_at ?? '~').localeCompare(b.run?.started_at ?? '~'));
			if (!entries.length && date !== data.today) continue;
			const monday = mondayOf(date);
			if (out.at(-1)?.monday !== monday) out.push({ monday, dates: [] });
			out.at(-1)!.dates.push({ date, entries });
		}
		return out;
	});
	const sessionsById = $derived(new Map([...data.library.map((t) => [t.id, t] as const), ...data.templates]));

	function weekTitle(monday: string) {
		if (monday === data.monday) return 'This week';
		return monday > data.monday ? 'Next week' : 'Last week';
	}

	// The week's dates, its cycles, and how it went or what is coming.
	function weekNote(monday: string) {
		const end = addDays(monday, 6);
		const days = data.days.filter((d) => monday <= d.local_date && d.local_date <= end);
		const cycles = new Set(days.flatMap((d) => data.cycles.filter((c) => c.id === d.cycle).map((c) => c.name)));
		const ran = finished.filter((r) => monday <= r.local_date && r.local_date <= end).length;
		const count =
			monday > data.monday
				? `${days.length} planned`
				: days.length
					? `${days.filter((d) => d.status === 'done').length} of ${days.length} done`
					: plural(ran, 'session');
		return [`${shortDate(monday)} – ${shortDate(end)}`, ...cycles, count].filter(Boolean).join(' · ');
	}

	function sectionsOf(template: string) {
		const n = sessionsById.get(template)?.sections.length ?? 0;
		return n === 1 ? '1 section' : n ? `${n} sections` : '';
	}

	const soFar = (startedAt: string) => formatDuration(Math.max(1, Math.floor(secondsSince(startedAt, now) / 60)) * 60);

	const stop = $derived(live ? currentStop(live) : null);
	const at = $derived(live && stop ? live.sections.findIndex((s) => s.items.includes(stop)) : -1);

	const bs = $derived(buckets(tenWeeks(data.today), data.today).buckets);
	const time = $derived(hours(finished, bs));
	// The usual week is the mean of the full weeks before this one that have
	// any training, once there are three of them.
	const trained = $derived(time.slice(0, -1).filter((h) => h > 0));
	const usual = $derived(trained.length >= 3 ? trained.reduce((a, b) => a + b, 0) / trained.length : null);
	const tenths = (n: number) => (Math.round(n * 10) / 10).toFixed(1);

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

	const empty = $derived(!other && !live && !planned.length && !done.length && !data.next && !offline);
</script>

<svelte:head><title>Today</title></svelte:head>

<div
	class="pointer-events-none absolute inset-x-0 top-0 -z-0 h-[420px] bg-[radial-gradient(120%_70%_at_100%_-10%,var(--glow-1),transparent_62%),radial-gradient(90%_60%_at_-10%_18%,var(--glow-2),transparent_62%)]"
></div>
<Topo
	shape={topTopo}
	class="absolute inset-x-0 top-0 h-80 w-full text-[var(--topo)] [mask-image:linear-gradient(#000_30%,transparent)]"
/>

<div class="relative pt-[env(safe-area-inset-top)] lg:grid lg:max-w-[1280px] lg:grid-cols-[minmax(0,1fr)_460px]">
	{@render days()}
	<div class="flex flex-col lg:border-l lg:border-line lg:pt-3">
		<header class="flex items-start justify-between gap-3 px-4 pt-3 {other ? '' : 'lg:hidden'}">
			<div>
				<h1 class="page-title">
					{other ? formatDate(shown, { weekday: 'long' }) : 'Today'}
				</h1>
				<p class="mt-1 text-[15px] font-semibold text-ink-2">
					{other ? `${formatDate(shown, { day: 'numeric', month: 'long' })} · ${away(daysBetween(data.today, shown))}` : date}
				</p>
			</div>
			<div class="flex min-w-0 items-center gap-2 pt-1.5">
				{#if other}
					<a
						href="/"
						class="flex h-11 items-center gap-1.5 rounded-full bg-surface px-4 text-[15px] font-bold text-ink shadow-card-sm"
						data-sveltekit-replacestate
						data-sveltekit-noscroll
					>
						<Icon name="undo-2" size="1.125rem" />Today
					</a>
				{/if}
				<a href="/profile" class="flex size-11 items-center justify-center rounded-full bg-surface text-ink shadow-card-sm md:hidden" aria-label="Profile">
					<Icon name="settings" size="1.25rem" />
				</a>
			</div>
		</header>

		<div class="px-4"><FormError message={error} /></div>

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

		<section class="mx-4 mt-3.5 rounded-3xl bg-surface pt-4 shadow-card {missed.length ? 'pb-1' : 'pb-4'} lg:hidden">
			<div class="flex items-start justify-between gap-3 px-5">
				<div class="min-w-0">
					<h2 class="text-[15px] font-bold">This week</h2>
					{#if cycle && progress}<p class="mt-0.5 truncate text-xs font-semibold text-ink-2">{cycle.name} · Week {progress.week} of {progress.of}</p>{/if}
				</div>
				<p class="shrink-0 text-[15px] font-semibold text-ink-2">
					{#if offline}
						No signal
					{:else if weekPlanned}
						<b class="digits text-ink">{weekDone} of {weekPlanned}</b> done{extra ? ` · ${extra} extra` : ''}
					{:else}
						<b class="digits text-ink">{ranThisWeek.length}</b> {ranThisWeek.length === 1 ? 'session' : 'sessions'} · {ranLastWeek} last week
					{/if}
				</p>
			</div>
			<WeekStrip monday={data.monday} today={data.today} week={data.week} selected={shown} ran={ranThisWeek.map((r) => r.local_date)} />
			{#if missed.length}
				<ul class="mt-3 px-5">
					{#each missed as d (d.id)}
						<li class="list-row">
							<a href="/plan" class="flex min-h-[60px] items-center gap-3">
								<span class="min-w-0 flex-1">
									<span class="block truncate text-[15px] font-bold">{d.template_name}</span>
									<span class="block text-xs font-semibold text-ink-2">Missed {formatDate(d.local_date, { weekday: 'long' })}</span>
								</span>
								<span class="flex items-center gap-0.5 text-xs font-bold text-ink">Plan<Icon name="chevron-right" size="14px" /></span>
							</a>
						</li>
					{/each}
				</ul>
			{/if}
		</section>

		{#if live && !other}
			{@render liveSlot(live)}
		{:else}
			{#each planned as d, i (d.id)}
				{@render sessionSlot(d, !other && i === 0)}
			{/each}
			{#if done.length}
				<section class="mx-4 mt-3.5 rounded-3xl bg-surface p-5 shadow-card">
					<p class="text-xs font-bold tracking-[0.08em] text-ink-2 uppercase">{other ? 'Done' : 'Done today'}</p>
					<ul class="mt-1">
						{#each done as r (r.id)}
							<li class="list-row">
								<a href="/history/{r.id}" class="flex min-h-[60px] items-center gap-3">
									<span class="min-w-0 flex-1 truncate text-xl font-bold tracking-tight">{r.name}</span>
									{#if r.elapsed_seconds}<span class="digits text-xl">{formatMinutes(r.elapsed_seconds)}</span>{/if}
								</a>
								{#if r.has_journal === false}
									<a href="/run/{r.id}/finish" class="-mt-3 flex min-h-11 w-fit items-center gap-0.5 text-xs font-bold text-ink-2">
										Add a journal<Icon name="chevron-right" size="14px" />
									</a>
								{/if}
							</li>
						{/each}
					</ul>
					{#if !other && !planned.length && data.next}{@render nextUp(data.next)}{/if}
				</section>
			{:else if other && !planned.length}
				{#if !stale.length}
					<section class="mx-4 mt-3.5 rounded-3xl bg-surface p-5 shadow-card">
						<p class="text-[28px] leading-tight font-extrabold tracking-tight">Nothing on this day</p>
					</section>
				{/if}
			{:else if !planned.length}
				{#if offline}
					<section class="mx-4 mt-3.5 rounded-3xl bg-surface p-5 shadow-card">
						<p class="text-[28px] leading-tight font-extrabold tracking-tight">No signal</p>
						<p class="mt-1 text-[15px] font-semibold text-ink-2">The plan cannot load. A running session still works.</p>
					</section>
				{:else if data.next}
					<section class="mx-4 mt-3.5 rounded-3xl bg-surface p-5 shadow-card">
						<p class="text-xs font-bold tracking-[0.08em] text-ink-2 uppercase">Today</p>
						<p class="mt-1 text-[28px] leading-tight font-extrabold tracking-tight">Rest day</p>
						{@render nextUp(data.next)}
					</section>
				{:else}
					<section class="mx-4 mt-3.5 rounded-3xl bg-surface p-5 shadow-card">
						<p class="text-xs font-bold tracking-[0.08em] text-ink-2 uppercase">Today</p>
						<p class="mt-1 text-[28px] leading-tight font-extrabold tracking-tight">Nothing planned</p>
						<p class="mt-1 text-[15px] font-semibold text-ink-2">Log as you go, or plan the week ahead.</p>
						<div class="mt-4">
							<Button disabled={starting !== null} onclick={() => start({ name: 'Open session' })}>
								{starting === 'open' ? 'Starting…' : 'Open session'}
							</Button>
						</div>
						<div class="mt-2.5"><Button variant="secondary" href="/plan">Plan a session</Button></div>
					</section>
				{/if}
			{/if}
		{/if}

		{#each stale as r (r.id)}
			<section class="mx-4 mt-3.5 rounded-3xl bg-surface p-5 shadow-card">
				<div class="flex items-start justify-between gap-3">
					<div class="min-w-0">
						<p class="text-xs font-bold tracking-[0.08em] text-ink-2 uppercase">Not finished</p>
						<p class="mt-1 truncate text-xl font-extrabold tracking-tight">{r.name}</p>
						<p class="mt-1 text-[15px] font-semibold text-ink-2">Started {dayDate(r.local_date)}. It counts once it is finished.</p>
					</div>
					<Menu look="bg-well text-ink" items={[{ label: 'Discard session', danger: true, onclick: () => discard(r) }]} />
				</div>
				<div class="mt-4"><Button variant="secondary" href="/run/{r.id}/finish">Finish session</Button></div>
			</section>
		{/each}

		{#if !other && mine.length}
			<section class="mx-4 mt-3.5 rounded-3xl bg-surface px-5 pt-4 pb-1 shadow-card">
				<div class="flex items-baseline justify-between">
					<h2 class="text-[15px] font-bold">Your sessions</h2>
					<a href="/templates" class="-my-3 flex h-11 items-center gap-0.5 text-xs font-bold text-ink">Library<Icon name="chevron-right" size="14px" /></a>
				</div>
				<ul class="mt-1">
					{#each mine.slice(0, 4) as t (t.id)}
						<li class="list-row flex items-center gap-3">
							<a href="/templates/{t.id}" class="flex min-h-[60px] min-w-0 flex-1 items-center gap-3">
								<SessionIcon icon={t.icon} name={t.name} />
								<span class="min-w-0 flex-1">
									<span class="block truncate text-[15px] font-bold">{t.name}</span>
									<span class="block text-xs font-semibold text-ink-2">{doneWhen(t.id)}</span>
								</span>
							</a>
							{#if !live}
								<button
									type="button"
									class="flex size-11 shrink-0 items-center justify-center rounded-full bg-well text-ink disabled:opacity-50"
									aria-label="Start {t.name}"
									disabled={starting !== null}
									onclick={() => startMine(t.id)}
								>
									<Icon name="play" size="1rem" />
								</button>
							{/if}
						</li>
					{/each}
				</ul>
			</section>
		{/if}

		{#if !offline}
			<section class="mx-4 mt-3.5 rounded-3xl bg-surface p-5 shadow-card">
				<div class="flex items-baseline justify-between">
					<h2 class="text-[15px] font-bold">Hours a week</h2>
					<a href="/history/progress" class="-my-3 flex h-11 items-center gap-0.5 text-xs font-bold text-ink">Progress<Icon name="chevron-right" size="14px" /></a>
				</div>
				<p class="mt-2 flex items-baseline gap-1.5">
					<b class="digits text-[32px] leading-none">{tenths(time.at(-1) ?? 0)}</b>
					<span class="text-[15px] font-semibold text-ink-2">
						{#if usual === null}h this week{:else}h of a usual <b class="digits text-ink">{tenths(usual)}</b> h week{/if}
					</span>
				</p>
				<div class="mt-4">
					<Bars parts={[{ values: time, cls: 'bg-chart' }]} height={44} small />
				</div>
				<p class="mt-1.5 text-xs font-semibold text-ink-3">Last 10 weeks</p>
			</section>
		{/if}

		{#if !live && !empty && !other && !offline}
			<section class="mx-4 mt-3.5 rounded-3xl bg-surface shadow-card">
				<button
					type="button"
					class="flex min-h-[60px] w-full items-center gap-3 px-5 text-left disabled:opacity-50"
					disabled={starting !== null}
					onclick={() => start({ name: 'Open session' })}
				>
					<span class="flex size-8 shrink-0 items-center justify-center rounded-full bg-well"><Icon name="plus" size="1.125rem" /></span>
					<span class="min-w-0 flex-1">
						<span class="block text-[15px] font-bold">{starting === 'open' ? 'Starting…' : 'Open session'}</span>
						<span class="block text-xs font-semibold text-ink-2">Log as you go</span>
					</span>
				</button>
			</section>
		{/if}
	</div>
</div>

{#snippet days()}
	<div class="hidden flex-col gap-3.5 px-8 pt-3 pb-8 lg:flex">
		<header>
			<h1 class="page-title">Today</h1>
			<p class="mt-1 text-[15px] font-semibold text-ink-2">{fullDate(data.today)}</p>
		</header>
		{#if offline}
			<p class="rounded-3xl bg-surface p-[18px] text-[15px] font-semibold text-ink-2 shadow-card">No signal. The days need one to load.</p>
		{:else}
			{#each listed as w (w.monday)}
				<section class="flex flex-col gap-2">
					<div class="flex items-baseline justify-between gap-3 px-1 pt-1.5">
						<h2 class="shrink-0 text-xl font-bold">{weekTitle(w.monday)}</h2>
						<span class="truncate text-xs font-semibold text-ink-2">{weekNote(w.monday)}</span>
					</div>
					<ul class="rounded-3xl bg-surface px-[18px] py-1 shadow-card">
						{#each w.dates as g (g.date)}
							{#each g.entries as e, i (e.run?.id ?? e.plan?.id)}
								<li class="list-row flex items-center gap-3.5">
									{#if e.run}
										{@const r = e.run}
										{@const open = r.finished_at === null}
										{@const left = data.stale.some((s) => s.id === r.id)}
										<a
											href={left ? `/run/${r.id}/finish` : open ? `/run/${r.id}` : `/history/${r.id}`}
											class="flex min-h-[58px] min-w-0 flex-1 items-center gap-3.5 py-2"
										>
											{@render dayCell(g.date, i === 0)}
											<SessionIcon icon={sessionsById.get(r.template ?? '')?.icon ?? null} name={r.name} state={open ? 'started' : 'done'} size={36} />
											<span class="min-w-0 flex-1">
												<span class="block truncate text-[15px] font-bold">{r.name}</span>
												<span class="mt-0.5 flex items-center gap-2 truncate text-xs font-semibold text-ink-2">
													{#if left}
														Not finished
													{:else if open}
														<i class="live-dot scale-75"></i>Running
													{:else}
														{r.place ?? ''}
													{/if}
												</span>
											</span>
											{#if open && !left}
												<span class="digits shrink-0 text-xl">{soFar(r.started_at)}</span>
											{:else if r.elapsed_seconds}
												<span class="digits shrink-0 text-xl">{formatMinutes(r.elapsed_seconds)}</span>
											{/if}
										</a>
										{#if !open && r.has_journal === false}
											<a href="/run/{r.id}/finish" class="hidden min-h-11 shrink-0 items-center gap-0.5 text-xs font-bold text-ink-2 xl:flex">
												Add a journal<Icon name="chevron-right" size="14px" />
											</a>
										{/if}
									{:else if e.plan}
										{@const d = e.plan}
										{@const usual = d.status === 'planned' ? factsOf(d).usual : null}
										<a
											href={d.status === 'missed' ? '/plan' : `/templates/${d.template}`}
											class="flex min-h-[58px] min-w-0 flex-1 items-center gap-3.5 py-2"
										>
											{@render dayCell(g.date, i === 0)}
											<SessionIcon icon={d.template_icon} name={d.template_name} state={iconState(d, data.today)} size={36} />
											<span class="min-w-0 flex-1">
												<span class="block truncate text-[15px] font-bold {d.status === 'missed' ? 'text-ink-2' : ''}">{d.template_name}</span>
												<span class="mt-0.5 block truncate text-xs font-semibold text-ink-2">
													{d.status === 'missed' ? 'Missed' : sectionsOf(d.template)}
												</span>
											</span>
											{#if d.status === 'missed'}
												<span class="flex shrink-0 items-center gap-0.5 text-xs font-bold text-ink">Plan<Icon name="chevron-right" size="14px" /></span>
											{:else if usual}
												<span class="shrink-0 text-xs font-semibold text-ink-3">Usually {usual}</span>
											{/if}
										</a>
									{/if}
								</li>
							{:else}
								<li class="list-row flex min-h-[58px] items-center gap-3.5 py-2">
									{@render dayCell(g.date, true)}
									<span class="text-[15px] font-semibold text-ink-2">Nothing yet</span>
								</li>
							{/each}
						{/each}
					</ul>
				</section>
			{/each}
		{/if}
	</div>
{/snippet}

{#snippet dayCell(date: string, first: boolean)}
	<span class="flex w-11 shrink-0 flex-col">
		{#if first}
			{#if date === data.today}
				<span class="text-xs font-bold text-ink">Today</span>
			{:else}
				<span class="text-xs font-semibold text-ink-2">{formatDate(date, { weekday: 'short' })}</span>
			{/if}
			<b class="text-xl leading-[1.05] font-bold">{formatDate(date, { day: 'numeric' })}</b>
		{/if}
	</span>
{/snippet}

{#snippet facts(usual: string | null, since: number | null)}
	{#if usual || since !== null}
		<div class="mt-3 flex gap-8">
			{#if usual}
				<p class="flex flex-col"><b class="digits text-xl">{usual}</b><span class="text-xs font-semibold text-ink-2">Usually</span></p>
			{/if}
			{#if since !== null}
				<p class="flex flex-col">
					<b class="digits text-xl">{plural(since, 'day')}</b><span class="text-xs font-semibold text-ink-2">Since last time</span>
				</p>
			{/if}
		</div>
	{:else}
		<p class="mt-1 text-[15px] font-semibold text-ink-2">First time</p>
	{/if}
{/snippet}

{#snippet sessionSlot(d: ScheduledDay, primary: boolean)}
	{@const f = factsOf(d)}
	<section class="mx-4 mt-3.5 rounded-3xl bg-surface p-5 shadow-card">
		<p class="flex items-center gap-2 text-xs font-bold tracking-[0.08em] text-ink-2 uppercase">
			<i class="size-2 rounded-full" style:background={data.templates.get(d.template)?.color ?? 'var(--ink-3)'}></i>
			{other ? (d.status === 'missed' ? 'Missed' : 'Planned') : 'Today'}
		</p>
		<a href="/templates/{d.template}" class="mt-1 block truncate text-[28px] leading-tight font-extrabold tracking-tight">{d.template_name}</a>
		{@render facts(f.usual, f.since)}
		<div class="mt-4">
			{#if d.status === 'started' && d.run}
				<Button variant="live" href="/run/{d.run}">Back to session</Button>
			{:else}
				<Button variant={primary ? 'primary' : 'secondary'} disabled={starting !== null} onclick={() => start({ scheduled: d.id })}>
					{starting === d.id ? 'Starting…' : primary ? 'Start' : 'Start now'}
				</Button>
			{/if}
		</div>
	</section>
{/snippet}

{#snippet nextUp(d: ScheduledDay)}
	{@const f = factsOf(d)}
	<div class="mt-4 rounded-2xl bg-well p-4">
		<p class="flex items-center gap-2 text-xs font-bold tracking-[0.08em] text-ink-2 uppercase">
			<i class="size-2 rounded-full" style:background={data.templates.get(d.template)?.color ?? 'var(--ink-3)'}></i>
			Next · {when(d.local_date)}
		</p>
		<a href="/templates/{d.template}" class="mt-1 block truncate text-xl font-extrabold tracking-tight">{d.template_name}</a>
		{@render facts(f.usual, f.since)}
		<div class="mt-4">
			<Button variant="secondary" disabled={starting !== null} onclick={() => start({ scheduled: d.id })}>
				{starting === d.id ? 'Starting…' : 'Start now'}
			</Button>
		</div>
	</div>
{/snippet}

{#snippet liveSlot(run: Run)}
	<section class="mx-4 mt-3.5 rounded-3xl bg-surface p-5 shadow-card">
		<p class="flex items-center gap-2.5 pl-1 text-xs font-bold tracking-[0.08em] text-ink-2 uppercase"><i class="live-dot"></i>Running</p>
		<p class="mt-1 truncate text-[28px] leading-tight font-extrabold tracking-tight">{run.name}</p>
		<div class="mt-3 flex gap-8">
			<p class="flex flex-col">
				<b class="digits text-xl">{soFar(run.started_at)}</b>
				<span class="text-xs font-semibold text-ink-2">So far</span>
			</p>
			{#if at >= 0}
				<p class="flex min-w-0 flex-col">
					<b class="digits text-xl">{at + 1} of {run.sections.length}</b>
					<span class="truncate text-xs font-semibold text-ink-2">{run.sections[at].name}</span>
				</p>
			{/if}
		</div>
		<div class="mt-4"><Button variant="live" href="/run/{run.id}">Back to session</Button></div>
	</section>
{/snippet}
