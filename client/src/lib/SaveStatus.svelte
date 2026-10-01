<script lang="ts">
	import { openRun } from './runState.svelte';
</script>

{#if openRun.stalled && openRun.writes.length}
	<div class="flex items-center justify-between gap-3 rounded-3xl bg-surface py-2 pr-2 pl-4 text-xs font-semibold text-ink-2 shadow-card-sm" role="status">
		<span>Not saved yet. It is kept on this phone.</span>
		<button type="button" class="h-11 rounded-full bg-well px-4 text-[15px] font-bold text-link" onclick={() => openRun.flush()}>Retry</button>
	</div>
{/if}
{#if openRun.refused}
	<div class="flex items-center justify-between gap-3 rounded-3xl bg-surface py-2 pr-2 pl-4 text-xs font-semibold text-bad shadow-card-sm" role="alert">
		<span>Not saved: {openRun.refused}</span>
		<button type="button" class="h-11 rounded-full bg-well px-4 text-[15px] font-bold text-ink" onclick={() => (openRun.refused = '')}>OK</button>
	</div>
{/if}
