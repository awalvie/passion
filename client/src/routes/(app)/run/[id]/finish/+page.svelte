<script lang="ts">
	import { goto } from '$app/navigation';
	import { openRun } from '$lib/runState.svelte';
	import SessionJournalForm from '$lib/SessionJournalForm.svelte';

	const run = $derived(openRun.run!);
</script>

<svelte:head><title>Session journal {run.name}</title></svelte:head>

<!-- Back from the saved run must not open the journal again. -->
<SessionJournalForm
	cancel={run.finished_at ? `/history/${run.id}` : `/run/${run.id}`}
	saved={(r) => goto(`/history/${r.id}`, { replaceState: true })}
/>
