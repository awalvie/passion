<script lang="ts">
	import { formatMinutes } from './exercise';
	import Icon from './Icon.svelte';
	import { notesOf, type DaySession } from './journal';
	import SessionIcon from './SessionIcon.svelte';

	// The sessions of a day, with what their own journals say, so a note reads
	// beside the training it is about. A run with no length is still running,
	// so it opens as a session.
	let { sessions }: { sessions: DaySession[] } = $props();
</script>

<ul class="rounded-3xl bg-surface px-[18px] py-2 shadow-card [--flush:-0.625rem]">
	{#each sessions as { run, journal } (run.id)}
		<li class="list-row flush">
			<a href={run.elapsed_seconds === null ? `/run/${run.id}` : `/history/${run.id}`} class="flex min-h-16 items-start gap-3 py-3">
				<SessionIcon icon={null} name={run.name} />
				<span class="min-w-0 flex-1 self-center">
					<span class="block truncate text-[15px] font-bold">{run.name}</span>
					<span class="block text-xs font-semibold text-ink-2">
						{[run.elapsed_seconds === null ? 'In progress' : formatMinutes(run.elapsed_seconds), journal?.rpe && `Effort ${journal.rpe}/10`]
							.filter(Boolean)
							.join(' · ')}
					</span>
					{#each journal ? notesOf(journal) : [] as [label, text] (label)}
						<span class="mt-2 block reading"><span class="font-semibold text-ink-2">{label}:</span> {text}</span>
					{/each}
				</span>
				<span class="mt-3 shrink-0 text-ink-3"><Icon name="chevron-right" size="18px" stroke={2.4} /></span>
			</a>
		</li>
	{/each}
</ul>
