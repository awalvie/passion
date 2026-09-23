<script lang="ts">
	import { distinct } from '$lib/exercise';
	import { urlFilters } from '$lib/filters.svelte';
	import Icon from '$lib/Icon.svelte';

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

<div class="card card-pad">
	<div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
		<div>
			<h1 class="text-xl font-bold">Your session templates</h1>
			<p class="mt-1 text-sm muted">Plan a session template (warm-up, exercises, cool-down) and reuse it.</p>
		</div>
		<a
			class="inline-flex shrink-0 items-center justify-center whitespace-nowrap rounded-md btn-primary px-4 py-2 text-sm font-medium"
			href="/templates/new"
		>
			New session template
		</a>
	</div>

	<div class="mt-4 flex flex-wrap items-center gap-2">
		<input
			type="search"
			bind:value={f.q}
			placeholder="Search by name..."
			aria-label="Search session templates"
			class="input text-sm w-full sm:w-auto"
			style="min-width:10rem"
			autocomplete="off"
		/>
		{#if sources.length}
			<select
				bind:value={f.source}
				aria-label="Source"
				class="input text-sm flex-1 min-w-0 sm:flex-none sm:w-auto sm:min-w-[9rem]"
			>
				<option value="">All sources</option>
				{#each sources as s (s)}
					<option value={s}>{s}</option>
				{/each}
			</select>
		{/if}
		{#if tags.length}
			<select
				bind:value={f.tag}
				aria-label="Labels"
				class="input text-sm flex-1 min-w-0 sm:flex-none sm:w-auto sm:min-w-[9rem]"
			>
				<option value="">All labels</option>
				{#each tags as t (t)}
					<option value={t}>{t}</option>
				{/each}
			</select>
		{/if}
		{#if filtered}
			<button type="button" class="rounded-md px-3 py-2 text-xs muted hover:underline" onclick={clear}>Clear</button>
		{/if}
		<span class="text-sm muted">{plural(shown.length, 'template')}</span>
	</div>

	{#if shown.length}
		<!-- Mobile: a card. Source and the section count share one truncating line. -->
		<div class="mt-6 space-y-2 md:hidden">
			{#each shown as t (t.id)}
				<div
					class="relative flex items-start gap-2 rounded-lg border px-4 py-3"
					style="border-color:var(--border);background:var(--panel);border-left:4px solid {t.color ??
						'var(--border)'}"
				>
					<a href="/templates/{t.id}" class="min-w-0 flex-1 no-underline after:absolute after:inset-0">
						<div class="text-sm font-medium truncate">{t.name}</div>
						{#if t.tags.length}
							<div class="mt-0.5 text-[11px] muted truncate">{t.tags.join(' · ')}</div>
						{/if}
						<div class="mt-0.5 text-[11px] muted truncate">
							{#if t.source}
								<span
									class="inline-flex items-center gap-1 align-middle font-medium px-1.5 py-0.5 rounded"
									style="background:var(--accent-bg);color:var(--accent)"
								>
									<Icon name="book-marked" size="0.6rem" />{t.source}
								</span>
							{/if}
							<span class="align-middle">{plural(t.sections.length, 'section')}</span>
						</div>
					</a>
				</div>
			{/each}
		</div>

		<!-- Desktop: a fixed table, so Source and Sections line up down the page. -->
		<div class="mt-6 hidden md:block">
			<table class="w-full text-sm table-fixed">
				<colgroup>
					<col style="width:0.75rem" />
					<col />
					<col style="width:15rem" />
					<col style="width:12rem" />
					<col style="width:5rem" />
				</colgroup>
				<thead>
					<tr class="text-left text-xs font-semibold muted">
						<th class="py-2"><span class="sr-only">Colour</span></th>
						<th class="px-3 py-2">Name</th>
						<th class="px-3 py-2">Labels</th>
						<th class="px-3 py-2">Source</th>
						<th class="px-3 py-2">Sections</th>
					</tr>
				</thead>
				<tbody>
					{#each shown as t (t.id)}
						<tr class="divider relative">
							<td class="py-2 align-middle">
								<span
									class="block rounded-sm"
									style="width:0.25rem;height:1.25rem;background:{t.color ?? 'var(--border)'}"
									aria-hidden="true"
								></span>
							</td>
							<td class="px-3 py-2 align-middle">
								<a
									class="text-sm font-medium link hover:underline after:absolute after:inset-0"
									href="/templates/{t.id}">{t.name}</a
								>
							</td>
							<td class="px-3 py-2 align-middle text-[11px] muted truncate">{t.tags.join(' · ')}</td>
							<td class="px-3 py-2 align-middle">
								{#if t.source}
									<span
										class="inline-flex max-w-full items-center gap-1 truncate text-[11px] font-medium px-1.5 py-0.5 rounded"
										style="background:var(--accent-bg);color:var(--accent)"
									>
										<Icon name="book-marked" size="0.6rem" />{t.source}
									</span>
								{/if}
							</td>
							<td class="px-3 py-2 align-middle text-[11px] muted tabular-nums">{t.sections.length}</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{:else}
		<p class="px-3 py-6 text-center text-sm muted">No session templates match your search.</p>
	{/if}
</div>
