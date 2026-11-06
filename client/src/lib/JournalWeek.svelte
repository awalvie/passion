<script lang="ts">
	import { afterNavigate } from '$app/navigation';
	import type { Snippet } from 'svelte';
	import { addDays, fullDate, mondayOf, shortDate } from './dates';
	import DaySessions from './DaySessions.svelte';
	import { sessionsOn, type Journal, type JournalEntry } from './journal';
	import type { RunSummary } from './run';

	// The week of a note or a session journal: every note and session of it,
	// day by day, the picked one outlined with its actions. Reading on is a
	// scroll, not a button.
	let {
		date,
		on,
		journal,
		runs,
		href,
		actions,
		heading = 'h2'
	}: {
		date: string;
		on: string;
		journal: Journal;
		runs: RunSummary[] | null;
		href: (e: JournalEntry) => string;
		actions?: Snippet<[JournalEntry]>;
		heading?: 'h1' | 'h2';
	} = $props();

	const days = $derived.by(() => {
		const mon = mondayOf(date);
		return Array.from({ length: 7 }, (_, i) => addDays(mon, i))
			.map((d) => ({ date: d, entries: journal.entries.filter((e) => e.local_date === d), sessions: sessionsOn(d, runs, journal.sessions) }))
			.filter((d) => d.entries.length || d.sessions.length);
	});

	// The picked one comes into view when it is picked: after the router has
	// set the scroll of the new page.
	afterNavigate(() => {
		document.querySelector('[data-picked]')?.scrollIntoView({ block: matchMedia('(min-width: 64rem)').matches ? 'nearest' : 'start' });
	});

	const paras = (body: string) => body.split(/\n\s*\n/).map((p) => p.trim());
</script>

<svelte:element this={heading} class="page-title px-1 text-balance">Week of {shortDate(mondayOf(date))}</svelte:element>
{#each days as d (d.date)}
	<section class="flex flex-col gap-2">
		<svelte:element this={heading === 'h1' ? 'h2' : 'h3'} class="px-1 text-[15px] font-bold">{fullDate(d.date)}</svelte:element>
		{#each d.entries as x (x.id)}
			{#if x.id === on}
				<div class="flex scroll-mt-20 flex-col gap-2.5 rounded-3xl bg-surface p-[18px] shadow-card ring-[2.5px] ring-ink" data-picked aria-current="true">
					<div class="flex flex-col gap-2.5 reading">{#each paras(x.body) as p, i (i)}<p>{p}</p>{/each}</div>
					{#if actions}
						<div class="flex items-center gap-2 pt-1">
							{#if x.pinned_at}<span class="text-xs font-semibold text-ink-2">Pinned</span>{/if}
							<div class="ml-auto">{@render actions(x)}</div>
						</div>
					{/if}
				</div>
			{:else}
				<a href={href(x)} data-sveltekit-replacestate data-sveltekit-noscroll class="flex flex-col gap-2.5 rounded-3xl bg-surface p-[18px] shadow-card reading">
					{#each paras(x.body) as p, i (i)}<p>{p}</p>{/each}
				</a>
			{/if}
		{/each}
		{#if d.sessions.length}
			{@const hit = d.sessions.some((x) => x.run.id === on)}
			<div class="scroll-mt-20 rounded-3xl {hit ? 'ring-[2.5px] ring-ink' : ''}" data-picked={hit || undefined} aria-current={hit || undefined}>
				<DaySessions sessions={d.sessions} />
			</div>
		{/if}
	</section>
{/each}
