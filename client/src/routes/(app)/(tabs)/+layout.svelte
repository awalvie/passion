<script lang="ts">
	import { page } from '$app/state';
	import PullRefresh from '$lib/PullRefresh.svelte';
	import TabBar from '$lib/TabBar.svelte';

	let { data, children } = $props();

	// Creating a cycle is a task with its own bottom button, so it hides the
	// tabs and the session strip.
	const task = $derived(page.url.pathname === '/plan/cycles/new');
	// Today shows the open session as a card, so it needs no strip.
	const live = $derived(page.url.pathname === '/' ? null : data.live);
</script>

<PullRefresh />

<!-- --above-bar is where a bar stuck to the bottom of a page, or a toast, sits
     clear of the tab bar and the open session's strip. -->
<div
	class={task ? '' : live ? 'pb-[calc(7.75rem+env(safe-area-inset-bottom))]' : 'pb-[calc(5rem+env(safe-area-inset-bottom))]'}
	style:--above-bar={task
		? 'calc(1rem + env(safe-area-inset-bottom))'
		: live
			? 'calc(7.5rem + env(safe-area-inset-bottom))'
			: 'calc(4.75rem + env(safe-area-inset-bottom))'}
>
	{@render children()}
</div>

{#if !task}
	<TabBar {live} />
{/if}
