<script lang="ts">
	import { distinct, originOf } from '$lib/exercise';
	import { urlFilters } from '$lib/filters.svelte';
	import FilterLine from '$lib/FilterLine.svelte';
	import Icon from '$lib/Icon.svelte';
	import LibraryFilters from '$lib/LibraryFilters.svelte';
	import LibrarySearch from '$lib/LibrarySearch.svelte';
	import OriginMark from '$lib/OriginMark.svelte';
	import { sessionIcons } from '$lib/SessionIcon.svelte';

	let { data } = $props();

	const f = urlFilters('/templates', ['q', 'origin', 'source', 'tag']);

	const sources = $derived(distinct(data.templates.flatMap((t) => (t.source ? [t.source] : []))));
	const tags = $derived(distinct(data.templates.flatMap((t) => t.tags)));

	const filters = $derived([f.origin, f.source, f.tag].filter(Boolean).length);

	const shown = $derived.by(() => {
		const needle = f.q.trim().toLowerCase();
		return data.templates.filter(
			(t) =>
				t.name.toLowerCase().includes(needle) &&
				(!f.origin || originOf(t) === f.origin) &&
				(!f.source || t.source === f.source) &&
				(!f.tag || t.tags.includes(f.tag))
		);
	});

	function clear() {
		f.q = f.origin = f.source = f.tag = '';
	}
</script>

<svelte:head><title>Session templates</title></svelte:head>

<div class="flex flex-col gap-3.5">
	<div class="flex items-center gap-2">
		<div class="min-w-0 flex-1"><LibrarySearch bind:value={f.q} placeholder="Search sessions" label="Search session templates" /></div>
		<LibraryFilters bind:origin={f.origin} bind:source={f.source} bind:tag={f.tag} {sources} {tags} shown={shown.length} noun="session" />
	</div>

	{#if (f.q.trim() || filters) && shown.length}
		<FilterLine shown={shown.length} total={data.templates.length} {filters} onclear={clear} />
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
		<div class="flex flex-col items-center gap-3 rounded-3xl bg-surface p-[18px] text-center text-[15px] font-semibold text-ink-2 shadow-card">
			{#if f.q.trim() || filters}
				Nothing matches.
				<button type="button" class="h-11 rounded-full bg-well px-5 text-[15px] font-bold text-ink" onclick={clear}>Clear</button>
			{:else}
				No session templates yet.
			{/if}
		</div>
	{/if}
</div>
