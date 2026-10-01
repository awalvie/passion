<script lang="ts">
	import { goto, invalidateAll } from '$app/navigation';
	import { describe, request } from '$lib/api';
	import { addDays, cycleWeek } from '$lib/dates';
	import type { Exercise } from '$lib/exercise';
	import ExercisePicker from '$lib/ExercisePicker.svelte';
	import FormError from '$lib/FormError.svelte';
	import Icon from '$lib/Icon.svelte';
	import LiveCard from '$lib/LiveCard.svelte';
	import type { ScheduledDay } from '$lib/plan';
	import { openRun, startRun, type StartBody } from '$lib/runState.svelte';
	import SessionCard from '$lib/SessionCard.svelte';
	import Sheet from '$lib/Sheet.svelte';
	import Topo from '$lib/Topo.svelte';
	import { topTopo } from '$lib/topo';
	import { bestWeight, track } from '$lib/tracked';
	import WeekStrip from '$lib/WeekStrip.svelte';

	let { data } = $props();

	let starting = $state(false);
	let error = $state('');

	async function start(body: StartBody) {
		starting = true;
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
			starting = false;
		}
	}

	const live = $derived(data.live && openRun.run?.id === data.live.id ? openRun.run : null);
	const days = $derived(data.days.filter((d) => d.run !== live?.id));

	async function discard() {
		error = '';
		try {
			await openRun.discard();
			await invalidateAll();
		} catch (e) {
			error = describe(e, (f) => f);
		}
	}

	const date = $derived(
		new Date(`${data.today}T00:00:00Z`).toLocaleDateString(undefined, {
			weekday: 'long',
			day: 'numeric',
			month: 'long',
			timeZone: 'UTC'
		})
	);

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

	function label(d: ScheduledDay) {
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
			<h1 class="text-[32px] leading-[1.1] font-extrabold tracking-[-0.02em]">Today</h1>
			<p class="mt-1 text-[15px] font-semibold text-ink-2">{date}</p>
		</div>
		<div class="flex min-w-0 items-center gap-2 pt-1.5">
			{#if progress && progress.of <= 8}
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
			<a href="/settings" class="flex size-11 items-center justify-center rounded-full bg-surface text-ink shadow-card-sm" aria-label="Settings">
				<Icon name="settings" size="1.25rem" />
			</a>
		</div>
	</header>

	<WeekStrip monday={data.monday} today={data.today} week={data.week} />

	<div class="px-4"><FormError message={error} /></div>

	{#if live}
		<LiveCard run={live} ondiscard={discard} />
	{/if}

	{#each days as d (d.id)}
		<SessionCard
			day={d}
			template={data.templates.get(d.template)}
			label={label(d)}
			{starting}
			onstart={() => start({ scheduled: d.id })}
		/>
	{/each}

	{#if !live && !days.length}
		<section class="mx-4 mt-3.5 flex flex-col gap-3 rounded-3xl bg-surface p-5 shadow-card">
			<p class="text-xl font-bold tracking-tight">
				{data.offline ? 'No signal' : 'Nothing planned today'}
			</p>
			<p class="text-[15px] font-semibold text-ink-2">
				{data.offline
					? 'The plan cannot load. A running session still works.'
					: 'Plan a session, or start one from the library.'}
			</p>
			{#if !data.offline}
				<a href="/plan" class="flex h-12 items-center justify-center rounded-full bg-tint text-[15px] font-bold text-on-tint shadow-tint">
					Plan a session
				</a>
			{/if}
		</section>
	{/if}

	{#if !data.live}
		<div class="flex gap-3 px-4 pt-3">
			<a href="/templates" class="flex h-12 flex-1 items-center gap-2.5 rounded-full bg-surface px-2 text-[15px] font-bold shadow-card">
				<span class="flex size-8 items-center justify-center rounded-full bg-well"><Icon name="stack" size="1.125rem" /></span>
				Other session
			</a>
			<button
				type="button"
				class="flex h-12 flex-1 items-center gap-2.5 rounded-full bg-surface px-2 text-[15px] font-bold shadow-card disabled:opacity-50"
				disabled={starting}
				onclick={() => start({ name: 'Open session' })}
			>
				<span class="flex size-8 items-center justify-center rounded-full bg-well"><Icon name="plus" size="1.125rem" /></span>
				Open session
			</button>
		</div>
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

{#snippet stat(value: string, unit: string, caption: string)}
	<div class="flex min-h-[92px] flex-col justify-between gap-2 rounded-[22px] bg-surface p-3.5 shadow-card">
		<p class="text-[32px] leading-[1.1] font-extrabold tracking-tight">
			{value}{#if unit}<small class="ml-[3px] text-[15px] font-semibold tracking-normal text-ink-2">{unit}</small>{/if}
		</p>
		<p class="text-xs font-semibold text-ink-2">{caption}</p>
	</div>
{/snippet}
