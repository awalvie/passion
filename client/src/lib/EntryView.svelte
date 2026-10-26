<script lang="ts">
	import { fullDate } from './dates';
	import { formatMinutes } from './exercise';
	import Icon from './Icon.svelte';
	import type { JournalEntry } from './journal';
	import SessionIcon from './SessionIcon.svelte';

	// An entry as it reads: its day, its text, and the session it was written
	// during. Beside the Journal's list, its title is an h2 under the page's h1.
	let { entry, heading = 'h1' }: { entry: JournalEntry; heading?: 'h1' | 'h2' } = $props();
</script>

<header class="px-1">
	{#if entry.pinned_at}<p class="text-xs font-bold tracking-[0.08em] text-ink-2 uppercase">Pinned</p>{/if}
	<svelte:element this={heading} class="page-title mt-1 text-balance">{fullDate(entry.local_date)}</svelte:element>
</header>

<div class="flex flex-col gap-2.5 rounded-3xl bg-surface px-5 py-[18px] shadow-card reading">
	{#each entry.body.split(/\n\s*\n/) as p, i (i)}<p>{p.trim()}</p>{/each}
</div>

{#if entry.run}
	<!-- A run with no length is still running, so it opens as a session. -->
	<a
		href={entry.run_elapsed_seconds === null ? `/run/${entry.run}` : `/history/${entry.run}`}
		class="flex min-h-16 items-center gap-3 rounded-3xl bg-surface px-4 py-3 shadow-card"
	>
		<SessionIcon icon={null} name={entry.run_name ?? ''} />
		<span class="min-w-0 flex-1">
			<span class="block truncate text-[15px] font-bold">{entry.run_name}</span>
			<span class="block text-xs font-semibold text-ink-2">
				{entry.run_elapsed_seconds === null ? 'In progress' : formatMinutes(entry.run_elapsed_seconds)}
			</span>
		</span>
		<span class="text-ink-3"><Icon name="chevron-right" size="18px" stroke={2.4} /></span>
	</a>
{/if}
