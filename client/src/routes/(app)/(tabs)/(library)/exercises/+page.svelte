<script lang="ts">
	import { distinct, kindOf, kinds, sourcesOf } from '$lib/exercise';
	import { urlFilters } from '$lib/filters.svelte';
	import Icon from '$lib/Icon.svelte';
	import LibrarySearch from '$lib/LibrarySearch.svelte';

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

	<!-- Two selects to a row on a phone: four on one row clip their own text. -->
	<div class="flex flex-wrap items-center gap-2">
		<select bind:value={f.kind} aria-label="Type" class="input min-h-10 w-[calc(50%-0.25rem)] min-w-0 rounded-full px-4 py-0 font-bold sm:w-auto">
			<option value="">All types</option>
			{#each kinds as k (k.kind)}
				<option value={k.kind}>{k.label}</option>
			{/each}
		</select>
		{#if sources.length}
			<select bind:value={f.source} aria-label="Source" class="input min-h-10 w-[calc(50%-0.25rem)] min-w-0 rounded-full px-4 py-0 font-bold sm:w-auto">
				<option value="">All sources</option>
				{#each sources as s (s)}
					<option value={s}>{s}</option>
				{/each}
			</select>
		{/if}
		{#if tags.length}
			<select bind:value={f.tag} aria-label="Labels" class="input min-h-10 w-[calc(50%-0.25rem)] min-w-0 rounded-full px-4 py-0 font-bold sm:w-auto">
				<option value="">All labels</option>
				{#each tags as t (t)}
					<option value={t}>{t}</option>
				{/each}
			</select>
		{/if}
		{#if filtered}
			<button type="button" class="flex h-10 items-center rounded-full bg-surface px-4 text-[15px] font-bold text-link shadow-card-sm" onclick={clear}>
				Clear
			</button>
		{/if}
	</div>

	<div class="flex items-center justify-between px-1 pt-1">
		<h2 class="text-xs font-semibold text-ink-2">Exercise library</h2>
		<span class="text-xs font-semibold text-ink-2">{shown.length} {shown.length === 1 ? 'exercise' : 'exercises'}</span>
	</div>

	{#if shown.length}
		<ul class="rounded-3xl bg-surface py-1 shadow-card">
			{#each shown as e (e.id)}
				<li class="[&:not(:first-child)]:shadow-[inset_0_1px_0_var(--line)]">
					<a href="/exercises/{e.id}" class="flex min-h-16 items-center gap-3.5 py-2.5 pr-3 pl-3.5">
						<span class="sr-only">{kindOf(e.kind).label}. </span>
						<span class="flex size-10 shrink-0 items-center justify-center rounded-[14px] bg-well text-ink" title={kindOf(e.kind).label}>
							<Icon name={kindOf(e.kind).icon} size="1.25rem" />
						</span>
						<span class="min-w-0 flex-1">
							<span class="block truncate text-[15px] font-bold">{e.name}</span>
							{#if e.tags.length || e.source}
								<span class="mt-0.5 block truncate text-xs font-semibold text-ink-2">
									{[...e.tags, e.source].filter(Boolean).join(' · ')}
								</span>
							{/if}
						</span>
						<span class="flex shrink-0 text-ink-3"><Icon name="chevron-right" size="18px" stroke={2.2} /></span>
					</a>
				</li>
			{/each}
		</ul>
	{:else}
		<p class="rounded-3xl bg-surface p-[18px] text-center text-[15px] font-semibold text-ink-2 shadow-card">No exercises match your search.</p>
	{/if}
</div>
