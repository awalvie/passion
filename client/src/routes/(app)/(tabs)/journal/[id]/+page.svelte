<script lang="ts">
	import { goto, invalidateAll } from '$app/navigation';
	import { page } from '$app/state';
	import { describe } from '$lib/api';
	import { fullDate } from '$lib/dates';
	import FormError from '$lib/FormError.svelte';
	import { cameFrom, deleteEntry, pinEntry, type JournalEntry } from '$lib/journal';
	import JournalWeek from '$lib/JournalWeek.svelte';
	import Menu from '$lib/Menu.svelte';
	import NavBar from '$lib/NavBar.svelte';

	let { data } = $props();

	const entry = $derived(data.entry);
	let error = $state('');

	const from = $derived(page.url.searchParams.get('from'));
	const back = $derived(cameFrom(from));
	const keep = $derived(from ? `?from=${encodeURIComponent(from)}` : '');

	// The week reads from the journal, so a pin reloads it.
	async function pin(e: JournalEntry, on: boolean) {
		error = '';
		try {
			await pinEntry(e, on);
			await invalidateAll();
		} catch (err) {
			error = describe(err, (f) => f);
		}
	}

	async function remove(e: JournalEntry) {
		error = '';
		try {
			if (await deleteEntry(e)) await goto(back.href, { replaceState: true });
		} catch (err) {
			error = describe(err, (f) => f);
		}
	}
</script>

<svelte:head><title>{fullDate(entry.local_date)}</title></svelte:head>

{#snippet actions(e: JournalEntry)}
	<div class="flex shrink-0 gap-2">
		<a href="/journal/{e.id}/edit{keep}" class="flex h-11 items-center rounded-full bg-well px-4 text-[15px] font-bold text-ink">Edit</a>
		<Menu
			look="bg-well text-ink"
			items={[
				{ label: e.pinned_at ? 'Unpin' : 'Pin', onclick: () => pin(e, !e.pinned_at) },
				{ label: 'Delete entry', danger: true, onclick: () => remove(e) }
			]}
		/>
	</div>
{/snippet}

<NavBar {back} />

<div class="flex flex-col gap-3.5 px-4 pt-2 pb-6">
	<FormError message={error} />
	<JournalWeek
		date={entry.local_date}
		on={entry.id}
		journal={data.journal}
		runs={data.runs}
		href={(x) => `/journal/${x.id}${keep}`}
		{actions}
		heading="h1"
	/>
</div>
