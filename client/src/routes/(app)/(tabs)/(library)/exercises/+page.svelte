<script lang="ts">
	import { distinct, kindOf, kinds, originOf, sourcesOf } from '$lib/exercise';
	import FilterSelect from '$lib/FilterSelect.svelte';
	import { urlFilters } from '$lib/filters.svelte';
	import Icon from '$lib/Icon.svelte';
	import LibrarySearch from '$lib/LibrarySearch.svelte';
	import OriginMark from '$lib/OriginMark.svelte';

	let { data } = $props();

	const f = urlFilters('/exercises', ['q', 'kind', 'source', 'tag']);

	const sources = $derived(sourcesOf(data.exercises));
	const tags = $derived(distinct(data.exercises.flatMap((e) => e.tags)));

	const shown = $derived.by(() => {
		const needle = f.q.trim().toLowerCase();
		return data.exercises.filter(
			(e) =>
				e.name.toLowerCase().includes(needle) &&
				(!f.kind || e.kind === f.kind) &&
				(!f.source || e.source === f.source) &&
				(!f.tag || e.tags.includes(f.tag))
		);
	});

	const filtered = $derived(Boolean(f.q || f.kind || f.source || f.tag));

	function clear() {
		f.q = f.kind = f.source = f.tag = '';
	}
</script>

<svelte:head><title>Exercises</title></svelte:head>

<div class="flex flex-col gap-3.5">
	<LibrarySearch bind:value={f.q} placeholder="Search exercises" label="Search exercises" />

	<!-- Two selects to a row: four on one row clip their own text. -->
	<div class="flex flex-wrap items-center gap-2">
		<FilterSelect bind:value={f.kind} label="Type" all="All types" options={kinds.map((k) => ({ value: k.kind, label: k.label }))} class="w-[calc(50%-0.25rem)]" />
		{#if sources.length}
			<FilterSelect bind:value={f.source} label="Source" all="All sources" options={sources.map((s) => ({ value: s, label: s }))} class="w-[calc(50%-0.25rem)]" />
		{/if}
		{#if tags.length}
			<FilterSelect bind:value={f.tag} label="Labels" all="All labels" options={tags.map((t) => ({ value: t, label: t }))} class="w-[calc(50%-0.25rem)]" />
		{/if}
		{#if filtered}
			<button type="button" class="flex h-11 items-center rounded-full bg-surface px-4 text-[15px] font-bold text-ink shadow-card-sm" onclick={clear}>
				Clear
			</button>
		{/if}
	</div>

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
