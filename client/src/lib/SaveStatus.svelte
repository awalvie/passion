<script lang="ts">
	import { openRun } from './runState.svelte';
</script>

{#if openRun.stalled && openRun.writes.length}
	<div class="flex items-center justify-between gap-3 rounded-xl bg-surface px-4 py-2 text-sm shadow-sm" role="status">
		<span>Not saved yet. It is kept on this phone.</span>
		<button type="button" class="font-semibold text-tint" onclick={() => openRun.flush()}>Retry</button>
	</div>
{/if}
{#if openRun.refused}
	<div class="flex items-center justify-between gap-3 rounded-xl bg-surface px-4 py-2 text-sm text-bad shadow-sm" role="alert">
		<span>Not saved: {openRun.refused}</span>
		<button type="button" class="font-semibold" onclick={() => (openRun.refused = '')}>OK</button>
	</div>
{/if}
