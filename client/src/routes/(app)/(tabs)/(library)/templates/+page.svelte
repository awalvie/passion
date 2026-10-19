<script lang="ts">
	import { distinct, originOf } from '$lib/exercise';
	import FilterSelect from '$lib/FilterSelect.svelte';
	import { urlFilters } from '$lib/filters.svelte';
	import Icon from '$lib/Icon.svelte';
	import LibrarySearch from '$lib/LibrarySearch.svelte';
	import OriginMark from '$lib/OriginMark.svelte';
	import { sessionIcons } from '$lib/SessionIcon.svelte';

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
				<FilterSelect bind:value={f.source} label="Source" all="All sources" options={sources.map((s) => ({ value: s, label: s }))} class="max-w-full flex-1" />
			{/if}
			{#if tags.length}
				<FilterSelect bind:value={f.tag} label="Labels" all="All labels" options={tags.map((t) => ({ value: t, label: t }))} class="max-w-full flex-1" />
			{/if}
			{#if filtered}
				<button type="button" class="flex h-11 items-center rounded-full bg-surface px-4 text-[15px] font-bold text-ink shadow-card-sm" onclick={clear}>Clear</button>
			{/if}
		</div>
	{/if}

	{#if shown.length}
		<ul class="rounded-3xl bg-surface py-1 shadow-card">
			{#each shown as t (t.id)}
				{@const icon = sessionIcons.find(([name]) => name === t.icon)?.[0]}
				<li class="list-row">
					<a href="/templates/{t.id}" class="flex min-h-[52px] items-center gap-3 py-2 pr-3.5 pl-3">
						<span
							class="flex size-8 shrink-0 items-center justify-center rounded-[11px] text-[13px] font-extrabold text-ink {t.color ? '' : 'bg-well'}"
							style={t.color ? `background: color-mix(in oklab, ${t.color} 22%, var(--surface))` : undefined}
							aria-hidden="true"
						>
							{#if icon}<Icon name={icon} size="1rem" />{:else}{t.name.trim().charAt(0).toUpperCase()}{/if}
						</span>
						<span class="line-clamp-2 min-w-0 flex-1 text-[15px] leading-snug font-bold">{t.name}</span>
						<OriginMark origin={originOf(t)} />
					</a>
				</li>
			{/each}
		</ul>
	{:else}
		<p class="rounded-3xl bg-surface p-[18px] text-center text-[15px] font-semibold text-ink-2 shadow-card">No session templates match your search.</p>
	{/if}
</div>
