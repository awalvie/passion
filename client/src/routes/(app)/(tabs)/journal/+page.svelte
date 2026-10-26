<script lang="ts">
	import Button from '$lib/Button.svelte';
	import { dayDate, formatDate, weekday } from '$lib/dates';
	import { urlFilters } from '$lib/filters.svelte';
	import FilterLine from '$lib/FilterLine.svelte';
	import Icon from '$lib/Icon.svelte';
	import { excerpt, notesOf, pieces, pinnedOf, rows, type Row, type SessionJournal } from '$lib/journal';
	import LibrarySearch from '$lib/LibrarySearch.svelte';

	let { data } = $props();

	const f = urlFilters('/journal', ['q']);
	const searching = $derived(!!f.q.trim());

	const pinned = $derived(searching ? [] : pinnedOf(data.journal.entries));
	const total = $derived(data.journal.entries.length + data.journal.sessions.length);
	const shown = $derived(rows(data.journal, f.q));

	const months = $derived.by(() => {
		const out: { label: string; rows: Row[] }[] = [];
		for (const r of shown) {
			const label = formatDate(r.date, { month: 'long', year: 'numeric' });
			if (out.at(-1)?.label !== label) out.push({ label, rows: [] });
			out.at(-1)!.rows.push(r);
		}
		return out;
	});

	// A session journal shows the note that matched, or its first one.
	function noteOf(s: SessionJournal) {
		const notes = notesOf(s);
		const q = f.q.trim().toLowerCase();
		return (q && notes.find(([, t]) => t.toLowerCase().includes(q))) || notes[0];
	}
</script>

<svelte:head><title>Journal</title></svelte:head>

{#snippet marked(text: string)}
	{#each pieces(excerpt(text, f.q), f.q) as p, i (i)}{#if p.hit}<mark class="rounded bg-well px-0.5 font-extrabold text-ink">{p.text}</mark>{:else}{p.text}{/if}{/each}
{/snippet}

{#snippet chevron()}
	<span class="shrink-0 text-ink-3"><Icon name="chevron-right" size="18px" stroke={2.4} /></span>
{/snippet}

<div class="flex flex-col gap-3.5 px-4 pt-[calc(env(safe-area-inset-top)+0.75rem)] pb-6">
	<header class="flex items-start justify-between gap-3">
		<h1 class="page-title">Journal</h1>
		{#if total}
			<a href="/journal/new" class="mt-1 flex size-11 items-center justify-center rounded-full bg-surface text-ink shadow-card-sm" aria-label="New entry">
				<Icon name="plus" size="1.25rem" />
			</a>
		{/if}
	</header>

	{#if !total}
		<section class="flex flex-col gap-3 rounded-3xl bg-surface p-5 shadow-card">
			<p class="text-xl font-bold tracking-tight">No entries yet</p>
			<p class="text-[15px] font-semibold text-ink-2">Write what you notice.</p>
			<Button href="/journal/new">New entry</Button>
		</section>
	{:else}
		<LibrarySearch bind:value={f.q} placeholder="Search journal" label="Search journal" />

		{#if searching && shown.length}
			<FilterLine shown={shown.length} {total} filters={0} onclear={() => (f.q = '')} />
		{/if}

		{#if pinned.length}
			<section class="rounded-3xl bg-surface px-[18px] pt-4 pb-2 shadow-card">
				<h2 class="mb-1 text-[15px] font-bold">Pinned</h2>
				<ul>
					{#each pinned as e (e.id)}
						<li class="list-row">
							<a href="/journal/{e.id}" class="flex items-center gap-3.5 py-2.5">
								<span class="min-w-0 flex-1">
									<span class="line-clamp-3 text-[15px] leading-[1.55] font-medium">{e.body}</span>
									<span class="mt-0.5 block truncate text-xs font-semibold text-ink-2">
										{[dayDate(e.local_date), e.run_name && `During ${e.run_name}`].filter(Boolean).join(' · ')}
									</span>
								</span>
								{@render chevron()}
							</a>
						</li>
					{/each}
				</ul>
			</section>
		{/if}

		{#each months as m (m.label)}
			<section class="rounded-3xl bg-surface px-[18px] pt-4 pb-2 shadow-card">
				<h2 class="mb-1 text-[15px] font-bold">{m.label}</h2>
				<ul>
					{#each m.rows as r (r.entry?.id ?? r.session!.run)}
						<li class="list-row">
							<a href={r.entry ? `/journal/${r.entry.id}` : `/history/${r.session!.run}`} class="flex min-h-16 items-center gap-3.5 py-2.5">
								<span class="flex size-12 shrink-0 flex-col items-center justify-center self-start rounded-2xl bg-well">
									<span class="text-[10px] leading-none font-bold tracking-wide text-ink-2 uppercase">{weekday(r.date)}</span>
									<span class="mt-0.5 digits text-xl leading-none">{Number(r.date.slice(8))}</span>
								</span>
								<span class="min-w-0 flex-1">
									{#if r.entry}
										<span class="line-clamp-3 text-[15px] leading-[1.55] font-medium">{@render marked(r.entry.body)}</span>
									{:else}
										{@const s = r.session!}
										{@const note = noteOf(s)}
										<span class="block truncate text-[15px] font-bold">{@render marked(s.name)}</span>
										<span class="block text-xs font-semibold text-ink-2">
											{['Session journal', s.rpe && `Effort ${s.rpe}/10`].filter(Boolean).join(' · ')}
										</span>
										<span class="mt-0.5 line-clamp-2 text-[15px] leading-[1.55] font-medium text-ink-2">
											<b class="font-bold text-ink">{note[0]}:</b>
											{@render marked(note[1])}
										</span>
									{/if}
								</span>
								{@render chevron()}
							</a>
						</li>
					{/each}
				</ul>
			</section>
		{:else}
			{#if searching}
				<div class="flex flex-col items-center gap-3 rounded-3xl bg-surface p-[18px] text-center text-[15px] font-semibold text-ink-2 shadow-card">
					Nothing matches.
					<button type="button" class="h-11 rounded-full bg-well px-5 text-[15px] font-bold text-ink" onclick={() => (f.q = '')}>Clear</button>
				</div>
			{/if}
		{/each}
	{/if}
</div>
