<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import EntryForm from '$lib/EntryForm.svelte';

	let { data } = $props();

	// Back to the entry, which keeps where it was opened from.
	const from = $derived(page.url.searchParams.get('from'));
	const entryHref = $derived(`/journal/${data.entry.id}${from ? `?from=${encodeURIComponent(from)}` : ''}`);
</script>

<svelte:head><title>Edit entry</title></svelte:head>

<!-- The form copies the entry once, so another id needs a new form. -->
{#key data.entry.id}
	<EntryForm
		entry={data.entry}
		today={data.today}
		cancel={entryHref}
		saved={() => goto(entryHref, { replaceState: true })}
	/>
{/key}
