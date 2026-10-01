<script lang="ts">
	import { page } from '$app/state';
	import TabBar from '$lib/TabBar.svelte';

	let { data, children } = $props();

	// Today's card already leads back to its own running session.
	const onCard = $derived(
		page.url.pathname === '/' &&
			((page.data.days ?? []) as { run: string | null }[]).some((d) => d.run === data.live?.id)
	);
	const bar = $derived(data.live && !onCard);
</script>

<div class="{bar ?'pb-[calc(8.5rem+env(safe-area-inset-bottom))]' : 'pb-[calc(5rem+env(safe-area-inset-bottom))]'}">
	{@render children()}
</div>

{#if bar && data.live}
	<a
		href="/run/{data.live.id}"
		class="fixed inset-x-0 bottom-[calc(4.75rem+env(safe-area-inset-bottom))] z-30 mx-auto flex h-14 w-[calc(100%-2rem)] max-w-[398px] items-center gap-3 rounded-full bg-hero pr-2 pl-5 text-on-hero shadow-card"
	>
		<span class="size-2.5 shrink-0 rounded-full bg-live shadow-[0_0_0_5px_var(--live-halo)]"></span>
		<span class="min-w-0 flex-1 truncate text-[15px] font-bold">{data.live.name}</span>
		<span class="flex h-10 shrink-0 items-center rounded-full bg-live px-4 text-[15px] font-bold text-on-live">Back to session</span>
	</a>
{/if}

<TabBar live={!!data.live} />
