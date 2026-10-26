<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { describe } from '$lib/api';
	import { fullDate } from '$lib/dates';
	import EntryView from '$lib/EntryView.svelte';
	import FormError from '$lib/FormError.svelte';
	import { cameFrom, deleteEntry, pinEntry } from '$lib/journal';
	import Menu from '$lib/Menu.svelte';
	import NavBar from '$lib/NavBar.svelte';

	let { data } = $props();

	let entry = $derived(data.entry);
	let error = $state('');

	const from = $derived(page.url.searchParams.get('from'));
	const back = $derived(cameFrom(from));

	async function pin(on: boolean) {
		error = '';
		try {
			entry = await pinEntry(entry, on);
		} catch (e) {
			error = describe(e, (f) => f);
		}
	}

	async function remove() {
		error = '';
		try {
			if (await deleteEntry(entry)) await goto(back.href, { replaceState: true });
		} catch (e) {
			error = describe(e, (f) => f);
		}
	}
</script>

<svelte:head><title>{fullDate(entry.local_date)}</title></svelte:head>

<NavBar {back}>
	{#snippet actions()}
		<a
			href="/journal/{entry.id}/edit{from ? `?from=${encodeURIComponent(from)}` : ''}"
			class="flex h-11 items-center rounded-full bg-surface px-4 text-[15px] font-bold text-ink shadow-card-sm">Edit</a
		>
		<Menu
			items={[
				{ label: entry.pinned_at ? 'Unpin' : 'Pin', onclick: () => pin(!entry.pinned_at) },
				{ label: 'Delete entry', danger: true, onclick: remove }
			]}
		/>
	{/snippet}
</NavBar>

<div class="flex flex-col gap-3.5 px-4 pt-2 pb-6">
	<FormError message={error} />
	<EntryView {entry} />
</div>
