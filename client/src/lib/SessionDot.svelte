<script lang="ts">
	let {
		mark,
		small = false
	}: { mark: 'done' | 'skipped' | 'now' | 'todo'; small?: boolean } = $props();

	const labels = { done: 'Done', now: 'Current', todo: 'To do', skipped: 'Skipped' };
</script>

<span
	class="relative z-10 flex shrink-0 items-center justify-center {small ? 'size-6' : 'size-8'}"
	role="img"
	aria-label={labels[mark]}
>
	{#if mark === 'done'}
		<span class="absolute rounded-full bg-ink dark:bg-ink-2 {small ? 'inset-0.5' : 'inset-[5px]'}"></span>
		<svg viewBox="0 0 24 24" class="relative text-ground {small ? 'size-3' : 'size-3.5'}" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M5 12.5l4.5 4.5L19 7" /></svg>
	{:else if mark === 'now'}
		<span class="absolute inset-0 rounded-full {small ? '' : 'bg-ground'}"></span>
		<span class="absolute inset-0 rounded-full bg-live-halo"></span>
		<span class="relative rounded-full bg-live {small ? 'size-3' : 'size-[18px]'}"></span>
	{:else}
		<span
			class="absolute rounded-full border-ink-3 {small
				? 'inset-[5px] border-2 bg-surface'
				: 'inset-2 border-[2.5px] bg-ground'} {mark === 'skipped' ? 'border-dashed' : ''}"
		></span>
	{/if}
</span>
