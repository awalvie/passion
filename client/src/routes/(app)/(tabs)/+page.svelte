<script lang="ts">
	import { goto, invalidateAll } from '$app/navigation';
	import { page } from '$app/state';
	import { describe, request } from '$lib/api';
	import { addDays, cycleWeek, daysBetween, formatDate } from '$lib/dates';
	import { formatDuration, type Exercise } from '$lib/exercise';
	import ExercisePicker from '$lib/ExercisePicker.svelte';
	import FormError from '$lib/FormError.svelte';
	import Icon from '$lib/Icon.svelte';
	import LiveCard from '$lib/LiveCard.svelte';
	import type { ScheduledDay } from '$lib/plan';
	import { openRun, startRun, type StartBody } from '$lib/runState.svelte';
	import SessionCard from '$lib/SessionCard.svelte';
	import Sheet from '$lib/Sheet.svelte';
	import Topo from '$lib/Topo.svelte';
	import { heroTopo, topTopo } from '$lib/topo';
	import { bestWeight, track } from '$lib/tracked';
	import WeekStrip from '$lib/WeekStrip.svelte';

	let { data } = $props();

	// The day being started, or 'open'.
	let starting = $state<string | null>(null);
	let error = $state('');

	async function start(body: StartBody) {
		starting = 'scheduled' in body ? body.scheduled : 'open';
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

	// A tap on the week strip shows that day in place of today.
	const shown = $derived.by(() => {
		const day = page.url.searchParams.get('day');
		return Array.from({ length: 7 }, (_, i) => addDays(data.monday, i)).includes(day ?? '') ? day! : data.today;
	});
	const other = $derived(shown !== data.today);
	const shownDays = $derived(other ? data.week.filter((d) => d.local_date === shown) : data.days);

	const live = $derived(data.live && openRun.run?.id === data.live.id ? openRun.run : null);
	const days = $derived(shownDays.filter((d) => d.run !== live?.id && d.status !== 'done'));
	// A run belongs to the day it was done, and to the planned day it ran for.
	const done = $derived(
		(data.runs ?? []).filter(
			(r) =>
				r.finished_at !== null &&
				(r.local_date === shown || shownDays.some((d) => d.run === r.id))
		)
	);

	async function discard() {
		error = '';
		try {
			await openRun.discard();
			await invalidateAll();
		} catch (e) {
			error = describe(e, (f) => f);
		}
	}

	const date = $derived(formatDate(shown, { weekday: 'long', day: 'numeric', month: 'long' }));

	function away(n: number) {
		if (n === 1) return 'tomorrow';
		if (n === -1) return 'yesterday';
		return n > 0 ? `in ${n} days` : `${-n} days ago`;
	}

	// The cycle of today's first session, or else any cycle that covers today.
	const cycle = $derived(
		data.cycles.find((c) => c.id === data.days.find((d) => d.cycle)?.cycle) ??
			data.cycles.find((c) => c.starts <= data.today && data.today <= c.ends)
	);
	const progress = $derived(cycle ? cycleWeek(cycle.starts, cycle.ends, data.today) : null);

	const sessionsThisWeek = $derived(
		data.runs?.filter(
			(r) => r.finished_at !== null && data.monday <= r.local_date && r.local_date <= addDays(data.monday, 6)
		).length
	);

	let best = $derived(data.best);
	let library = $state<Exercise[]>([]);
	let choosing = $state(false);

	async function choose() {
		error = '';
		try {
			const { exercises } = await request<{ exercises: Exercise[] }>('GET', '/api/v1/exercises');
			library = exercises.filter((e) => !e.retired_at);
			choosing = true;
		} catch (e) {
			error = describe(e, (f) => f);
		}
	}

	async function follow(e: Exercise) {
		track(e.id);
		choosing = false;
		try {
			best = await bestWeight(e.id, data.cycleStarts!, data.today);
		} catch (err) {
			error = describe(err, (f) => f);
		}
	}

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

	function label(d: ScheduledDay) {
		if (other) {
			const when = formatDate(d.local_date, { weekday: 'short', day: 'numeric', month: 'short' });
			return `${d.status === 'missed' ? 'Missed' : 'Planned'} · ${when}`;
		}
		const c = data.cycles.find((x) => x.id === d.cycle);
		if (!c) return d.cycle ? 'Cycle' : 'One-off';
		return `${c.name} · Week ${cycleWeek(c.starts, c.ends, data.today).week}`;
	}
</script>

<svelte:head><title>Today</title></svelte:head>

<div
	class="pointer-events-none absolute inset-x-0 top-0 -z-0 h-[420px] bg-[radial-gradient(120%_70%_at_100%_-10%,var(--glow-1),transparent_62%),radial-gradient(90%_60%_at_-10%_18%,var(--glow-2),transparent_62%)]"
></div>
<Topo
	shape={topTopo}
	class="absolute inset-x-0 top-0 h-80 w-full text-[var(--topo)] [mask-image:linear-gradient(#000_30%,transparent)]"
/>

<div class="relative flex flex-col pt-[env(safe-area-inset-top)]">
	<header class="flex items-start justify-between gap-3 px-4 pt-3">
		<div>
			<h1 class="text-[32px] leading-[1.1] font-extrabold tracking-[-0.02em]">
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
			{:else if progress && progress.of <= 8}
				<div class="flex min-w-0 flex-col gap-1.5 rounded-[20px] bg-surface px-3 py-2 text-xs font-bold shadow-card-sm">
					<span>Week {progress.week} of {progress.of}</span>
					<span class="flex min-w-0 gap-[3px]">
						{#each Array.from({ length: progress.of }, (_, i) => i + 1) as w (w)}
							<i
								class="h-1 w-3.5 min-w-0 rounded-sm {w < progress.week
									? 'bg-ink dark:bg-tint'
									: w === progress.week
										? 'bg-[linear-gradient(90deg,var(--ink)_50%,var(--well)_50%)] dark:bg-[linear-gradient(90deg,var(--tint)_50%,var(--well)_50%)]'
										: 'bg-well'}"
							></i>
						{/each}
					</span>
				</div>
			{:else if progress}
				<span class="rounded-full bg-surface px-3 py-2 text-xs font-bold shadow-card-sm">Week {progress.week} of {progress.of}</span>
			{/if}
			<a href="/profile" class="flex size-11 items-center justify-center rounded-full bg-surface text-ink shadow-card-sm" aria-label="Profile">
				<Icon name="settings" size="1.25rem" />
			</a>
		</div>
	</header>

	<WeekStrip monday={data.monday} today={data.today} week={data.week} selected={shown} />

	<div class="px-4"><FormError message={error} /></div>

	{#if settingUp}
		<article
			class="relative mx-4 mt-3.5 overflow-hidden rounded-[32px] bg-[radial-gradient(90%_70%_at_88%_0%,var(--hero-2),var(--hero)_70%)] text-on-hero shadow-[0_18px_36px_-12px_rgba(10,30,18,0.55)]"
		>
			<Topo shape={heroTopo} class="absolute inset-0 h-full w-full text-[var(--hero-topo)]" />
			<div class="relative flex flex-col p-5">
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
			</div>
		</article>
	{/if}

	{#if live}
		<LiveCard run={live} ondiscard={discard} />
	{/if}

	{#each days as d (d.id)}
		<SessionCard
			day={d}
			template={data.templates.get(d.template)}
			label={label(d)}
			starting={starting !== null}
			busy={starting === d.id}
			startLabel={other ? 'Start now' : 'Start'}
			onstart={() => start({ scheduled: d.id })}
		/>
	{/each}

	{#each done as r (r.id)}
		<a href="/history/{r.id}" class="mx-4 mt-3 flex items-center gap-3 rounded-3xl bg-surface py-3 pr-3.5 pl-3 shadow-card">
			<span class="flex size-9 shrink-0 items-center justify-center rounded-full bg-ink text-tint">
				<Icon name="check" size="1.125rem" stroke={2.6} />
			</span>
			<span class="min-w-0 flex-1">
				<span class="block truncate text-[15px] font-bold">{r.name}</span>
				<span class="block text-xs font-semibold text-ink-2">
					Done{r.elapsed_seconds ? ` · ${formatDuration(Math.max(1, Math.round(r.elapsed_seconds / 60)) * 60)}` : ''}
				</span>
			</span>
			<span class="text-ink-3"><Icon name="chevron-right" size="18px" stroke={2.2} /></span>
		</a>
	{/each}

	{#if !live && !days.length && !done.length && !settingUp}
		<section class="mx-4 mt-3.5 flex flex-col gap-3 rounded-3xl bg-surface p-5 shadow-card">
			<p class="text-xl font-bold tracking-tight">
				{data.offline ? 'No signal' : other ? 'Nothing planned' : 'Nothing planned today'}
			</p>
			<p class="text-[15px] font-semibold text-ink-2">
				{data.offline
					? 'The plan cannot load. A running session still works.'
					: other
						? 'Nothing on this day.'
						: 'Plan a session, or start one from the library.'}
			</p>
			{#if !data.offline && !other}
				<a href="/plan" class="flex h-12 items-center justify-center rounded-full bg-tint text-[15px] font-bold text-on-tint shadow-tint">
					Plan a session
				</a>
			{/if}
		</section>
	{/if}

	{#if !data.live}
		<section class="mx-4 mt-3.5 rounded-3xl bg-surface py-0.5 shadow-card">
			<button
				type="button"
				class="flex min-h-[60px] w-full items-center gap-3 px-4 text-left disabled:opacity-50"
				disabled={starting !== null}
				onclick={() => start({ name: 'Open session' })}
			>
				{@render action('plus', starting === 'open' ? 'Starting…' : 'Open session', 'Log as you go')}
			</button>
			<a href="/templates" class="flex min-h-[60px] items-center gap-3 px-4 shadow-[inset_0_1px_0_var(--line)]">
				{@render action('stack', 'Other session', 'From the library')}
			</a>
		</section>
	{/if}

	{#if sessionsThisWeek !== undefined || data.cycleStarts}
		<div class="flex items-center justify-between px-5 pt-4 pb-2">
			<h2 class="text-xl font-bold">{data.cycleStarts ? 'This cycle' : 'This week'}</h2>
			{#if data.cycleStarts}
				<button type="button" class="-my-3.5 flex h-11 items-center gap-0.5 text-xs font-bold text-ink-2" onclick={choose}>
					Choose<Icon name="chevron-right" size="14px" />
				</button>
			{/if}
		</div>
		<div class="grid grid-cols-[1fr_1fr_1.25fr] gap-2.5 px-4">
			{#if sessionsThisWeek !== undefined}
				{@render stat(String(sessionsThisWeek), '', 'Sessions this week')}
			{/if}
			{#if data.sends !== null}
				{@render stat(String(data.sends), '', 'Sends this cycle')}
			{/if}
			{#if data.cycleStarts && best}
				{@render stat(best.kg === null ? '–' : String(best.kg), best.kg === null ? '' : 'kg', `Best ${best.name}`)}
			{:else if data.cycleStarts}
				<button
					type="button"
					class="flex min-h-[92px] flex-col items-start justify-between gap-2 rounded-[22px] bg-surface p-3.5 text-left shadow-card"
					onclick={choose}
				>
					<span class="flex size-8 items-center justify-center rounded-full bg-well"><Icon name="plus" size="1.125rem" /></span>
					<span class="text-xs font-semibold text-ink-2">Choose an exercise to see its best</span>
				</button>
			{/if}
		</div>
	{/if}
</div>

<Sheet bind:open={choosing} title="Track an exercise">
	<ExercisePicker id="track-exercise" label="Exercise" exercises={library} pick={follow} />
</Sheet>

{#snippet action(icon: 'plus' | 'stack', title: string, caption: string)}
	<span class="flex size-8 shrink-0 items-center justify-center rounded-full bg-well"><Icon name={icon} size="1.125rem" /></span>
	<span class="min-w-0 flex-1">
		<span class="block text-[15px] font-bold">{title}</span>
		<span class="block text-xs font-semibold text-ink-2">{caption}</span>
	</span>
	<span class="text-ink-3"><Icon name="chevron-right" size="18px" stroke={2.2} /></span>
{/snippet}

{#snippet stat(value: string, unit: string, caption: string)}
	<div class="flex min-h-[92px] flex-col justify-between gap-2 rounded-[22px] bg-surface p-3.5 shadow-card">
		<p class="text-[32px] leading-[1.1] font-extrabold tracking-tight">
			{value}{#if unit}<small class="ml-[3px] text-[15px] font-semibold tracking-normal text-ink-2">{unit}</small>{/if}
		</p>
		<p class="text-xs font-semibold text-ink-2">{caption}</p>
	</div>
{/snippet}
