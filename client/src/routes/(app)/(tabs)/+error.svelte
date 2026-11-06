<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import Button from '$lib/Button.svelte';
	import Empty from '$lib/Empty.svelte';
	import { tabOf, tabs } from '$lib/tabs';

	// Inside the tabs, so a page that fails to load keeps them.
	const lost = $derived(page.error?.message === 'No signal');
	// The tab's name keeps its place, as on the page that failed.
	const tab = $derived(tabs[tabOf(page.url.pathname)]?.label ?? '');
	const title = $derived(page.status === 404 ? 'Not found' : lost ? 'No signal' : 'Something went wrong');
</script>

<svelte:head><title>{title}</title></svelte:head>

<!-- Where a list tab puts its list, so the title stays where it was. -->
<div class="flex flex-col gap-3.5 px-4 pt-[calc(env(safe-area-inset-top)+0.75rem)] lg:mx-auto lg:max-w-[1280px] lg:px-6 lg:pt-3 lg:*:w-[372px]">
	{#if tab}<h1 class="page-title px-1">{tab}</h1>{/if}
	<Empty
		{title}
		text={page.status === 404 ? 'That page does not exist, or it is not yours.' : lost ? 'This page loads when you are back online.' : page.error?.message}
	>
		{#if page.status !== 404}
			<div class="lg:w-60"><Button onclick={() => goto(page.url, { invalidateAll: true })}>Try again</Button></div>
		{/if}
	</Empty>
</div>
