<script lang="ts">
	import TabBar from '$lib/TabBar.svelte';

	let { data, children } = $props();
</script>

<div class="{data.live ? 'pb-[calc(7rem+env(safe-area-inset-bottom)+1rem)]' : 'pb-[calc(3.5rem+env(safe-area-inset-bottom)+1rem)]'}">
	{@render children()}
</div>

{#if data.live}
	<a
		href="/run/{data.live.id}"
		class="fixed inset-x-0 bottom-[calc(3.5rem+env(safe-area-inset-bottom))] z-30 mx-auto flex h-14 w-full max-w-[430px] items-center gap-3 bg-surface px-4 shadow-[0_-1px_0_var(--line)]"
	>
		<span class="size-2.5 shrink-0 animate-pulse rounded-full bg-live"></span>
		<span class="min-w-0 flex-1 truncate text-base font-semibold">{data.live.name}</span>
		<span class="shrink-0 text-base font-semibold text-live">Back to session</span>
	</a>
{/if}

<TabBar />
