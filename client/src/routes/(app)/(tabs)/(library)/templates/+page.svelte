<script lang="ts">
	import { distinct } from '$lib/exercise';
	import FilterSelect from '$lib/FilterSelect.svelte';
	import { urlFilters } from '$lib/filters.svelte';
	import Icon from '$lib/Icon.svelte';
	import LibrarySearch from '$lib/LibrarySearch.svelte';
	import { plural } from '$lib/text';

	let { data } = $props();

	const f = urlFilters('/templates', ['q', 'source', 'tag']);

	const sources = $derived(distinct(data.templates.flatMap((t) => (t.source ? [t.source] : []))));
	const tags = $derived(distinct(data.templates.flatMap((t) => t.tags)));

	const shown = $derived.by(() => {
		const needle = f.q.trim().toLowerCase();
		return data.templates.filter(
			(t) =>
				t.name.toLowerCase().includes(needle) &&
				(!f.source || t.source === f.source) &&
				(!f.tag || t.tags.includes(f.tag))
		);
	});

	const filtered = $derived(Boolean(f.q || f.source || f.tag));

	function clear() {
		f.q = f.source = f.tag = '';
	}
</script>

<svelte:head><title>Session templates</title></svelte:head>

<div class="flex flex-col gap-3.5">
	<LibrarySearch bind:value={f.q} placeholder="Search sessions" label="Search session templates" />

	{#if sources.length || tags.length || filtered}
		<div class="flex flex-wrap items-center gap-2">
			{#if sources.length}
				<FilterSelect bind:value={f.source} label="Source" all="All sources" options={sources.map((s) => ({ value: s, label: s }))} class="max-w-full flex-1 sm:flex-none" />
			{/if}
			{#if tags.length}
				<FilterSelect bind:value={f.tag} label="Labels" all="All labels" options={tags.map((t) => ({ value: t, label: t }))} class="max-w-full flex-1 sm:flex-none" />
			{/if}
			{#if filtered}
				<button type="button" class="flex h-11 items-center rounded-full bg-surface px-4 text-[15px] font-bold text-ink shadow-card-sm" onclick={clear}>Clear</button>
			{/if}
		</div>
	{/if}

	<div class="flex items-baseline justify-between px-1 pt-1">
		<h2 class="text-xl font-bold">Your session templates</h2>
		<span class="text-xs font-semibold text-ink-2">{plural(shown.length, 'template')}</span>
	</div>

	{#if shown.length}
		<ul class="rounded-3xl bg-surface py-1 shadow-card">
			{#each shown as t (t.id)}
				<li class="list-row">
					<a href="/templates/{t.id}" class="flex min-h-16 items-center gap-3 py-2.5 pr-3 pl-[18px]">
						<span
							class="size-2.5 shrink-0 rounded-full {t.color ? '' : 'bg-well'}"
							style={t.color ? `background:${t.color}` : undefined}
							aria-hidden="true"
						></span>
						<span class="min-w-0 flex-1">
							<span class="block truncate text-[15px] font-bold">{t.name}</span>
							<span class="mt-0.5 block truncate text-xs font-semibold text-ink-2">
								{[plural(t.sections.length, 'section'), ...t.tags].join(' · ')}
							</span>
						</span>
						{#if t.source}
							<span class="inline-flex max-w-[40%] shrink-0 items-center gap-1 rounded-xl bg-well px-2.5 py-[5px] text-xs font-bold text-ink-2">
								<Icon name="book-marked" size="0.75rem" />
								<span class="truncate">{t.source}</span>
							</span>
						{/if}
						<span class="flex shrink-0 text-ink-3"><Icon name="chevron-right" size="18px" stroke={2.2} /></span>
					</a>
				</li>
			{/each}
		</ul>
	{:else}
		<p class="rounded-3xl bg-surface p-[18px] text-center text-[15px] font-semibold text-ink-2 shadow-card">No session templates match your search.</p>
	{/if}
</div>
