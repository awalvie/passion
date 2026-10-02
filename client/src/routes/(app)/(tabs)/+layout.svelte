<script lang="ts">
	import { notice } from '$lib/notice.svelte';
	import Toast from '$lib/Toast.svelte';
	import { invalidateAll } from '$app/navigation';
	import { page } from '$app/state';
	import PullRefresh from '$lib/PullRefresh.svelte';
	import TabBar from '$lib/TabBar.svelte';

	let { data, children } = $props();

	// Creating a cycle is a task with its own bottom button, so it hides the
	// tabs and the mini player.
	const task = $derived(page.url.pathname === '/plan/cycles/new');
	// Today shows the open session as a card, so it needs no mini player.
	const live = $derived(page.url.pathname === '/' ? null : data.live);

	// Back after a while away, the app fetches its pages again.
	$effect(() => {
		let away = 0;
		const back = () => {
			if (document.hidden) away = Date.now();
			else if (away && Date.now() - away > 2 * 60_000) void invalidateAll();
		};
		document.addEventListener('visibilitychange', back);
		return () => document.removeEventListener('visibilitychange', back);
	});
</script>

<PullRefresh />

<!-- --above-bar is where a bar stuck to the bottom of a page, or a toast, sits
     clear of the tab bar and the open session's mini player. -->
<div
	class={task ? '' : live ? 'pb-[calc(9.25rem+env(safe-area-inset-bottom))]' : 'pb-[calc(5rem+env(safe-area-inset-bottom))]'}
	style:--above-bar={task
		? 'calc(1rem + env(safe-area-inset-bottom))'
		: live
			? 'calc(9rem + env(safe-area-inset-bottom))'
			: 'calc(4.75rem + env(safe-area-inset-bottom))'}
>
	{@render children()}
	<Toast bind:message={notice.text} duration={2000} />
</div>

{#if !task}
	<TabBar {live} />
{/if}
