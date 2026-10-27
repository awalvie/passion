<script lang="ts">
	import { page } from '$app/state';
	import { MediaQuery } from 'svelte/reactivity';
	import { distinct, originOf } from './exercise';
	import { filterQuery, urlFilters } from './filters.svelte';
	import FilterLine from './FilterLine.svelte';
	import Icon from './Icon.svelte';
	import LibraryFilters from './LibraryFilters.svelte';
	import LibrarySearch from './LibrarySearch.svelte';
	import OriginMark from './OriginMark.svelte';
	import { doneWhen, lastDone, type RunSummary } from './run';
	import { sessionIcons } from './SessionIcon.svelte';
	import type { SessionTemplate } from './template';

	let { templates, runs, today }: { templates: SessionTemplate[]; runs: RunSummary[] | null; today: string } = $props();

	// From lg a row opens its item beside the list, keeping the filters.
	const wide = new MediaQuery('min-width: 64rem');
	const f = urlFilters(['q', 'origin', 'source', 'tag'], true);

	const sources = $derived(distinct(templates.flatMap((t) => (t.source ? [t.source] : []))));
	const tags = $derived(distinct(templates.flatMap((t) => t.tags)));

	const filters = $derived([f.origin, f.source, f.tag].filter(Boolean).length);

	// A built-in session you have never run says nothing, and with no runs to
	// go by, no session does.
	const ranOn = $derived(lastDone(runs ?? []));
	const done = (t: SessionTemplate) =>
		!runs || (t.shipped && !ranOn.has(t.id)) ? '' : doneWhen(ranOn.get(t.id), today);

	const shown = $derived.by(() => {
		const needle = f.q.trim().toLowerCase();
		return templates.filter(
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

<div class="flex flex-col gap-3.5">
	<div class="flex items-center gap-2">
		<div class="min-w-0 flex-1"><LibrarySearch bind:value={f.q} placeholder="Search sessions" label="Search session templates" /></div>
		<LibraryFilters bind:origin={f.origin} bind:source={f.source} bind:tag={f.tag} {sources} {tags} shown={shown.length} noun="session" />
	</div>

	{#if (f.q.trim() || filters) && shown.length}
		<FilterLine shown={shown.length} total={templates.length} {filters} onclear={clear} />
	{/if}

	{#if shown.length}
		<ul class="rounded-3xl bg-surface py-1 shadow-card">
			{#each shown as t (t.id)}
				{@const icon = sessionIcons.find(([name]) => name === t.icon)?.[0]}
				{@const when = done(t)}
				{@const picked = wide.current && page.params.id === t.id}
				<li class="list-row {picked ? 'picked' : ''}">
					<a
						href={wide.current ? `/templates/${t.id}${filterQuery(f)}` : `/templates/${t.id}`}
						data-sveltekit-replacestate={wide.current ? '' : undefined}
						data-sveltekit-noscroll={wide.current ? '' : undefined}
						aria-current={picked ? 'true' : undefined}
						class="flex min-h-[52px] items-center gap-3 py-2 pr-3.5 pl-3"
					>
						<span
							class="flex size-8 shrink-0 items-center justify-center rounded-[11px] text-[13px] font-extrabold text-ink {t.color ? '' : 'bg-well'}"
							style={t.color ? `background: color-mix(in oklab, ${t.color} 22%, var(--surface))` : undefined}
							aria-hidden="true"
						>
							{#if icon}<Icon name={icon} size="1rem" />{:else}{t.name.trim().charAt(0).toUpperCase()}{/if}
						</span>
						<span class="min-w-0 flex-1">
							<span class="line-clamp-2 text-[15px] leading-snug font-bold">{t.name}</span>
							{#if when}
								<span class="block text-xs font-semibold text-ink-2">{when}</span>
							{/if}
						</span>
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

<style>
	/* The open item's row beside the list, drawn as a well inside the card. */
	.picked {
		margin-inline: 0.375rem;
		border-radius: 1rem;
		background: var(--well);
		box-shadow: none;
	}
	.picked > a {
		padding-inline: 0.375rem 0.5rem;
	}
	.picked :global(.bg-well) {
		background: var(--surface);
	}
	.picked + :global(li) {
		box-shadow: none;
	}
</style>
