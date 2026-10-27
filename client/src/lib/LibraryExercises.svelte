<script lang="ts">
	import { page } from '$app/state';
	import { MediaQuery } from 'svelte/reactivity';
	import Button from './Button.svelte';
	import Empty from './Empty.svelte';
	import { distinct, kindOf, kinds, matches, originOf, sourcesOf, type Exercise } from './exercise';
	import { filterQuery, urlFilters } from './filters.svelte';
	import FilterLine from './FilterLine.svelte';
	import Icon from './Icon.svelte';
	import LibraryFilters from './LibraryFilters.svelte';
	import LibrarySearch from './LibrarySearch.svelte';
	import OriginMark from './OriginMark.svelte';
	import Tabs from './Tabs.svelte';

	let { exercises }: { exercises: Exercise[] } = $props();

	// From lg a row opens its item beside the list, keeping the filters.
	const wide = new MediaQuery('min-width: 64rem');
	const f = urlFilters(['q', 'kind', 'origin', 'source', 'tag'], true);

	const sources = $derived(sourcesOf(exercises));
	const tags = $derived(distinct(exercises.flatMap((e) => e.tags)));

	const section = $derived(exercises.filter((e) => !f.kind || e.kind === f.kind));
	const filters = $derived([f.origin, f.source, f.tag].filter(Boolean).length);

	const shown = $derived(section.filter((e) => matches(e, f)));

	function clear() {
		f.q = f.origin = f.source = f.tag = '';
	}
</script>

<div class="flex flex-col gap-3.5">
	<div class="flex items-center gap-2">
		<div class="min-w-0 flex-1"><LibrarySearch bind:value={f.q} placeholder="Search exercises" label="Search exercises" /></div>
		<LibraryFilters bind:origin={f.origin} bind:source={f.source} bind:tag={f.tag} {sources} {tags} shown={shown.length} noun="exercise" />
	</div>

	<Tabs bind:value={f.kind} label="Type" options={[{ value: '', label: 'All' }, ...kinds.map((k) => ({ value: k.kind, label: k.label }))]} />

	{#if (f.q.trim() || filters) && shown.length}
		<FilterLine shown={shown.length} total={section.length} {filters} onclear={clear} />
	{/if}

	{#if shown.length}
		<ul class="rounded-3xl bg-surface py-1 shadow-card">
			{#each shown as e (e.id)}
				{@const picked = page.params.id === e.id}
				<li class="list-row flush {picked ? 'picked-item' : ''}">
					<a
						href={wide.current ? `/exercises/${e.id}${filterQuery(f)}` : `/exercises/${e.id}`}
						aria-current={picked ? 'true' : undefined}
						class="flex min-h-[52px] items-center gap-3 py-2 pr-3.5 pl-3"
					>
						<span class="sr-only">{kindOf(e.kind).label}. </span>
						<span class="flex size-8 shrink-0 items-center justify-center rounded-[11px] bg-well text-ink" title={kindOf(e.kind).label}>
							<Icon name={kindOf(e.kind).icon} size="1rem" />
						</span>
						<span class="line-clamp-2 min-w-0 flex-1 text-[15px] leading-snug font-bold">{e.name}</span>
						<OriginMark origin={originOf(e)} />
						<span class="shrink-0 text-ink-3 lg:hidden"><Icon name="chevron-right" size="18px" stroke={2.4} /></span>
					</a>
				</li>
			{/each}
		</ul>
	{:else}
		{#if f.q.trim() || filters}
			<Empty title="Nothing matches">
				<button type="button" class="h-11 self-start rounded-full bg-well px-5 text-[15px] font-bold text-ink" onclick={clear}>Clear</button>
			</Empty>
		{:else}
			<Empty title={f.kind ? 'No exercises of this type yet' : 'No exercises yet'} text="Add one to build sessions from.">
				<Button href="/exercises/new">New exercise</Button>
			</Empty>
		{/if}
	{/if}
</div>
