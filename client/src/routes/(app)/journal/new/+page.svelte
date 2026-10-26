<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import EntryForm from '$lib/EntryForm.svelte';
	import { cameFrom } from '$lib/journal';

	let { data } = $props();

	// Save goes back to where the entry was started, the Journal or Today.
	const back = $derived(cameFrom(page.url.searchParams.get('from')));
</script>

<svelte:head><title>New entry</title></svelte:head>

<EntryForm today={data.today} cancel={back.href} saved={() => goto(back.href, { replaceState: true })} />
