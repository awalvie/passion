<script lang="ts">
	import { distinct } from '$lib/exercise';
	import { urlFilters } from '$lib/filters.svelte';
	import Icon from '$lib/Icon.svelte';
	import LibrarySearch from '$lib/LibrarySearch.svelte';

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

	function plural(n: number, word: string) {
		return `${n} ${word}${n === 1 ? '' : 's'}`;
	}
</script>

<svelte:head><title>Session templates</title></svelte:head>

<div class="flex flex-col gap-3.5">
	<LibrarySearch bind:value={f.q} placeholder="Search sessions" label="Search session templates" />

	{#if sources.length || tags.length || filtered}
		<div class="flex flex-wrap items-center gap-2">
			{#if sources.length}
				<select bind:value={f.source} aria-label="Source" class="input min-h-10 max-w-full min-w-0 flex-1 rounded-full px-4 py-0 font-bold sm:flex-none">
					<option value="">All sources</option>
					{#each sources as s (s)}
						<option value={s}>{s}</option>
					{/each}
				</select>
			{/if}
			{#if tags.length}
				<select bind:value={f.tag} aria-label="Labels" class="input min-h-10 max-w-full min-w-0 flex-1 rounded-full px-4 py-0 font-bold sm:flex-none">
					<option value="">All labels</option>
					{#each tags as t (t)}
						<option value={t}>{t}</option>
					{/each}
				</select>
			{/if}
			{#if filtered}
				<button type="button" class="flex h-10 items-center rounded-full bg-surface px-4 text-[15px] font-bold text-link shadow-card-sm" onclick={clear}>Clear</button>
			{/if}
		</div>
	{/if}

	<div class="flex items-center justify-between px-1 pt-1">
		<h2 class="text-xs font-semibold text-ink-2">Your session templates</h2>
		<span class="text-xs font-semibold text-ink-2">{plural(shown.length, 'template')}</span>
	</div>

	{#if shown.length}
		<ul class="rounded-3xl bg-surface py-1 shadow-card">
			{#each shown as t (t.id)}
				<li class="[&:not(:first-child)]:shadow-[inset_0_1px_0_var(--line)]">
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
						<svg viewBox="0 0 24 24" class="size-[18px] shrink-0 text-ink-3" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M9 6l6 6-6 6" /></svg>
					</a>
				</li>
			{/each}
		</ul>
	{:else}
		<p class="rounded-3xl bg-surface p-[18px] text-center text-[15px] font-semibold text-ink-2 shadow-card">No session templates match your search.</p>
	{/if}
</div>
