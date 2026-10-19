<script lang="ts">
	import { distinct, kindOf, kinds, originOf, sourcesOf } from '$lib/exercise';
	import { urlFilters } from '$lib/filters.svelte';
	import Icon from '$lib/Icon.svelte';
	import LibraryFilters from '$lib/LibraryFilters.svelte';
	import LibrarySearch from '$lib/LibrarySearch.svelte';
	import OriginMark from '$lib/OriginMark.svelte';
	import Tabs from '$lib/Tabs.svelte';

	let { data } = $props();

	const f = urlFilters('/exercises', ['q', 'kind', 'origin', 'source', 'tag']);

	const sources = $derived(sourcesOf(data.exercises));
	const tags = $derived(distinct(data.exercises.flatMap((e) => e.tags)));

	const shown = $derived.by(() => {
		const needle = f.q.trim().toLowerCase();
		return data.exercises.filter(
			(e) =>
				e.name.toLowerCase().includes(needle) &&
				(!f.kind || e.kind === f.kind) &&
				(!f.origin || originOf(e) === f.origin) &&
				(!f.source || e.source === f.source) &&
				(!f.tag || e.tags.includes(f.tag))
		);
	});
</script>

<svelte:head><title>Exercises</title></svelte:head>

<div class="flex flex-col gap-3.5">
	<div class="flex items-center gap-2">
		<div class="min-w-0 flex-1"><LibrarySearch bind:value={f.q} placeholder="Search exercises" label="Search exercises" /></div>
		<LibraryFilters bind:origin={f.origin} bind:source={f.source} bind:tag={f.tag} {sources} {tags} shown={shown.length} noun="exercise" />
	</div>

	<Tabs bind:value={f.kind} label="Type" options={[{ value: '', label: 'All' }, ...kinds.map((k) => ({ value: k.kind, label: k.label }))]} />

	{#if shown.length}
		<ul class="rounded-3xl bg-surface py-1 shadow-card">
			{#each shown as e (e.id)}
				<li class="list-row">
					<a href="/exercises/{e.id}" class="flex min-h-[52px] items-center gap-3 py-2 pr-3.5 pl-3">
						<span class="sr-only">{kindOf(e.kind).label}. </span>
						<span class="flex size-8 shrink-0 items-center justify-center rounded-[11px] bg-well text-ink" title={kindOf(e.kind).label}>
							<Icon name={kindOf(e.kind).icon} size="1rem" />
						</span>
						<span class="line-clamp-2 min-w-0 flex-1 text-[15px] leading-snug font-bold">{e.name}</span>
						<OriginMark origin={originOf(e)} />
					</a>
				</li>
			{/each}
		</ul>
	{:else}
		<p class="rounded-3xl bg-surface p-[18px] text-center text-[15px] font-semibold text-ink-2 shadow-card">No exercises match your search.</p>
	{/if}
</div>
