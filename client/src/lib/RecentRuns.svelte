<script lang="ts">
	import { request } from './api';
	import ExerciseHistory from './ExerciseHistory.svelte';
	import { loadGrades, type Scale } from './grades';
	import Icon from './Icon.svelte';
	import Section from './Section.svelte';
	import type { HistorySession } from './run';

	// An exercise's last three sessions, so its plan can be changed beside what
	// was done. History opens in a new tab, as the builder holds unsaved work.
	let { exercise, sides }: { exercise: string; sides: number } = $props();

	let found = $state<{ exercise: string; sessions: HistorySession[]; scales: Scale[] } | null>(null);
	let failed = $state(false);

	$effect(() => {
		const id = exercise;
		failed = false;
		Promise.all([request<{ sessions: HistorySession[] }>('GET', `/api/v1/exercises/${id}/history`), loadGrades()]).then(
			([h, g]) => {
				if (id === exercise) found = { exercise: id, sessions: h.sessions, scales: g.grades.scales };
			},
			() => {
				if (id === exercise) failed = true;
			}
		);
	});
	const shown = $derived(found?.exercise === exercise ? found : null);
</script>

<Section title="Last 3 runs">
	{#snippet aside()}
		<a
			href="/exercises/{exercise}"
			target="_blank"
			rel="noopener"
			class="flex h-11 items-center gap-1.5 rounded-full bg-well px-4 text-[15px] font-bold text-ink"
		>
			History<Icon name="external" size="1rem" />
		</a>
	{/snippet}
	{#if shown}
		<ExerciseHistory sessions={shown.sessions} scales={shown.scales} {sides} rows={3} chart={false} />
	{:else if failed}
		<p class="px-1 text-[15px] font-semibold text-ink-2">No signal. Past runs need one to load.</p>
	{:else}
		<p class="px-1 text-[15px] font-semibold text-ink-2">Loading…</p>
	{/if}
</Section>
