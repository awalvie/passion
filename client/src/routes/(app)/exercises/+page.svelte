<script lang="ts">
	import { replaceState } from '$app/navigation';
	import { page } from '$app/state';
	import { kindOf, kinds } from '$lib/exercise';
	import Icon from '$lib/Icon.svelte';

	let { data } = $props();

	// The filters live in the query string, so Back from an exercise finds them
	// as they were. Keys are V1's.
	const query = page.url.searchParams;
	let q = $state(query.get('q') ?? '');
	let kind = $state(query.get('kind') ?? '');
	let source = $state(query.get('source') ?? '');
	let tag = $state(query.get('tag') ?? '');

	const sources = $derived(distinct(data.exercises.flatMap((e) => (e.source ? [e.source] : []))));
	const tags = $derived(distinct(data.exercises.flatMap((e) => e.tags)));

	const shown = $derived.by(() => {
		const needle = q.trim().toLowerCase();
		return data.exercises.filter(
			(e) =>
				e.name.toLowerCase().includes(needle) &&
				(!kind || e.kind === kind) &&
				(!source || e.source === source) &&
				(!tag || e.tags.includes(tag))
		);
	});

	const filtered = $derived(Boolean(q || kind || source || tag));

	let written = search();
	$effect(() => {
		const next = search();
		if (next === written) return;
		written = next;
		replaceState(`/exercises${next}`, {});
	});

	function search() {
		const params = new URLSearchParams();
		for (const [key, value] of Object.entries({ q, kind, source, tag })) {
			if (value) params.set(key, value);
		}
		return params.size ? `?${params}` : '';
	}

	function distinct(values: string[]) {
		return [...new Set(values)].sort((a, b) => a.localeCompare(b));
	}

	function clear() {
		q = kind = source = tag = '';
	}
</script>

<svelte:head><title>Exercises</title></svelte:head>

<div class="card card-pad">
	<div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
		<div>
			<h1 class="text-xl font-bold">Exercise library</h1>
			<p class="mt-1 text-sm muted">The exercises you can add to any session template.</p>
		</div>
		<a
			class="inline-flex items-center justify-center rounded-md btn-primary px-4 py-2 text-sm font-medium"
			href="/exercises/new"
		>
			New exercise
		</a>
	</div>

	<!-- Search owns row 1. The selects sit two to a row below sm and inline above it,
	     because four on one phone row clip their own text. -->
	<div class="mt-4 flex flex-wrap items-end gap-2">
		<div class="w-full sm:flex-1" style="min-width:10rem">
			<input
				type="search"
				bind:value={q}
				placeholder="Search exercises..."
				aria-label="Search exercises"
				class="input text-sm"
				autocomplete="off"
			/>
		</div>
		<select
			bind:value={kind}
			aria-label="Type"
			class="input text-sm w-[calc(50%-0.25rem)] min-w-0 sm:w-auto sm:min-w-[8rem]"
		>
			<option value="">All types</option>
			{#each kinds as k (k.kind)}
				<option value={k.kind}>{k.label}</option>
			{/each}
		</select>
		{#if sources.length}
			<select
				bind:value={source}
				aria-label="Source"
				class="input text-sm w-[calc(50%-0.25rem)] min-w-0 sm:w-auto sm:min-w-[8rem]"
			>
				<option value="">All sources</option>
				{#each sources as s (s)}
					<option value={s}>{s}</option>
				{/each}
			</select>
		{/if}
		{#if tags.length}
			<select
				bind:value={tag}
				aria-label="Label"
				class="input text-sm w-[calc(50%-0.25rem)] min-w-0 sm:w-auto sm:min-w-[8rem]"
			>
				<option value="">All labels</option>
				{#each tags as t (t)}
					<option value={t}>{t}</option>
				{/each}
			</select>
		{/if}
		{#if filtered}
			<button type="button" class="rounded-md px-3 py-2 text-xs muted hover:underline" onclick={clear}>
				Clear
			</button>
		{/if}
	</div>

	{#if shown.length}
		<!-- Mobile: a two-line card with no source, because a 303px row cannot hold a
		     source chip and still show the name. -->
		<div class="mt-3 space-y-1.5 md:hidden">
			{#each shown as e (e.id)}
				<div
					class="relative rounded-lg border px-3 py-2"
					style="border-color:var(--border);background:var(--panel)"
				>
					<a href="/exercises/{e.id}" class="lib-row no-underline after:absolute after:inset-0">
						<span class="sr-only">{kindOf(e.kind).label}. </span>
						<span class="lib-row-glyph muted"><Icon name={kindOf(e.kind).icon} size="0.875rem" /></span>
						<span class="lib-row-name text-sm font-medium truncate">{e.name}</span>
						{#if e.tags.length}
							<span class="lib-row-labels text-[11px] muted truncate">{e.tags.join(' · ')}</span>
						{/if}
					</a>
				</div>
			{/each}
		</div>

		<!-- Desktop: a fixed table, so an empty cell still lines up its neighbours.
		     The source column fits the widest source on one line. -->
		<div class="mt-4 hidden md:block">
			<table class="w-full text-sm table-fixed">
				<colgroup>
					<col style="width:2rem" />
					<col />
					<col style="width:15rem" />
					<col style="width:12rem" />
				</colgroup>
				<thead>
					<tr class="text-left text-xs font-semibold muted">
						<th class="px-2 py-2"><span class="sr-only">Type</span></th>
						<th class="px-3 py-2">Name</th>
						<th class="px-3 py-2">Labels</th>
						<th class="px-3 py-2">Source</th>
					</tr>
				</thead>
				<tbody>
					{#each shown as e (e.id)}
						<tr class="divider relative">
							<td class="px-2 py-2 align-middle muted" title={kindOf(e.kind).label}>
								<Icon name={kindOf(e.kind).icon} size="0.875rem" />
							</td>
							<td class="px-3 py-2 align-middle">
								<a
									class="text-sm font-medium link hover:underline after:absolute after:inset-0"
									href="/exercises/{e.id}">{e.name}</a
								>
							</td>
							<td class="px-3 py-2 align-middle text-[11px] muted truncate">{e.tags.join(' · ')}</td>
							<td class="px-3 py-2 align-middle">
								{#if e.source}
									<span
										class="inline-flex max-w-full items-center gap-1 truncate text-[11px] font-medium px-1.5 py-0.5 rounded"
										style="background:var(--accent-bg);color:var(--accent)"
									>
										<Icon name="book-marked" size="0.6rem" />{e.source}
									</span>
								{/if}
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{:else}
		<p class="px-3 py-6 text-center text-sm muted">No exercises match your search.</p>
	{/if}
</div>
