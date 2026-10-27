<script lang="ts">
	import { goto, invalidateAll } from '$app/navigation';
	import { MediaQuery } from 'svelte/reactivity';
	import { describe } from '$lib/api';
	import Button from '$lib/Button.svelte';
	import { dayDate, formatDate, weekday } from '$lib/dates';
	import Empty from '$lib/Empty.svelte';
	import EntryForm from '$lib/EntryForm.svelte';
	import EntryView from '$lib/EntryView.svelte';
	import { urlFilters } from '$lib/filters.svelte';
	import FilterLine from '$lib/FilterLine.svelte';
	import FormError from '$lib/FormError.svelte';
	import Icon from '$lib/Icon.svelte';
	import {
		deleteEntry,
		excerpt,
		notesOf,
		pieces,
		pinEntry,
		pinnedOf,
		rows,
		type JournalEntry,
		type Row,
		type SessionJournal
	} from '$lib/journal';
	import LibrarySearch from '$lib/LibrarySearch.svelte';
	import Menu from '$lib/Menu.svelte';

	let { data } = $props();

	// A laptop opens an entry, New entry and Edit entry in a pane beside the
	// list, named by open, new and edit.
	const f = urlFilters(['q', 'open', 'new', 'edit']);
	const searching = $derived(!!f.q.trim());
	const wide = new MediaQuery('(min-width: 1024px)');

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

	// With nothing picked, the pane shows the first entry in the list.
	const opened = $derived.by(() => {
		if (!wide.current || f.new) return undefined;
		const id = f.edit || f.open;
		return (id && data.journal.entries.find((e) => e.id === id)) || pinned[0] || shown.find((r) => r.entry)?.entry;
	});
	const editing = $derived(!!f.edit && opened?.id === f.edit);

	let error = $state('');

	// pane is this page with the search kept and one entry, or New entry, open.
	function pane(key?: 'open' | 'new' | 'edit', id = '1') {
		const q = new URLSearchParams();
		if (f.q) q.set('q', f.q);
		if (key) q.set(key, id);
		return q.size ? `/journal?${q}` : '/journal';
	}

	const entryHref = (e: JournalEntry) => (wide.current ? pane('open', e.id) : `/journal/${e.id}`);

	async function show(href: string) {
		error = '';
		await goto(href, { replaceState: true, noScroll: true, invalidateAll: true });
	}

	async function pin(e: JournalEntry, on: boolean) {
		error = '';
		try {
			await pinEntry(e, on);
			await invalidateAll();
		} catch (err) {
			error = describe(err, (x) => x);
		}
	}

	async function remove(e: JournalEntry) {
		error = '';
		try {
			if (await deleteEntry(e)) await show(pane());
		} catch (err) {
			error = describe(err, (x) => x);
		}
	}

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

{#snippet chevron(cls = '')}
	<span class="shrink-0 text-ink-3 {cls}"><Icon name="chevron-right" size="18px" stroke={2.4} /></span>
{/snippet}

<div class="lg:mx-auto lg:grid lg:max-w-[1280px] lg:grid-cols-[420px_minmax(0,1fr)]">
	<div class="flex flex-col gap-3.5 px-4 pt-[calc(env(safe-area-inset-top)+0.75rem)] pb-6 lg:border-r lg:border-line lg:px-6 lg:pt-3">
		<header class="flex items-start justify-between gap-3">
			<h1 class="page-title px-1">Journal</h1>
			{#if total}
				<a
					href={wide.current ? pane('new') : '/journal/new'}
					data-sveltekit-replacestate={wide.current ? '' : undefined}
					data-sveltekit-noscroll={wide.current ? '' : undefined}
					class="flex size-11 items-center justify-center rounded-full bg-surface text-ink shadow-card-sm"
					aria-label="New entry"
				>
					<Icon name="plus" size="1.25rem" />
				</a>
			{/if}
		</header>

		{#if !total}
			<Empty title="No entries yet" text="Write what you notice.">
				<Button href={wide.current ? pane('new') : '/journal/new'}>New entry</Button>
			</Empty>
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
							{@const picked = opened?.id === e.id}
							<li class="list-row {picked ? 'picked' : ''}">
								<a
									href={entryHref(e)}
									data-sveltekit-replacestate={wide.current ? '' : undefined}
									data-sveltekit-noscroll={wide.current ? '' : undefined}
									aria-current={picked ? 'true' : undefined}
									class="flex items-center gap-3.5 py-2.5"
								>
									<span class="min-w-0 flex-1">
										<span class="line-clamp-3 text-[15px] leading-[1.55] font-medium">{e.body}</span>
										<span class="mt-0.5 block truncate text-xs font-semibold text-ink-2">
											{[dayDate(e.local_date), e.run_name && `During ${e.run_name}`].filter(Boolean).join(' · ')}
										</span>
									</span>
									{@render chevron('lg:hidden')}
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
							{@const picked = !!r.entry && opened?.id === r.entry.id}
							<li class="list-row {picked ? 'picked' : ''}">
								<a
									href={r.entry ? entryHref(r.entry) : `/history/${r.session!.run}`}
									data-sveltekit-replacestate={r.entry && wide.current ? '' : undefined}
									data-sveltekit-noscroll={r.entry && wide.current ? '' : undefined}
									aria-current={picked ? 'true' : undefined}
									class="flex min-h-16 items-center gap-3.5 py-2.5"
								>
									<span class="flex size-12 shrink-0 flex-col items-center justify-center self-start rounded-2xl {picked ? 'bg-surface' : 'bg-well'}">
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
									{@render chevron(r.entry ? 'lg:hidden' : '')}
								</a>
							</li>
						{/each}
					</ul>
				</section>
			{:else}
				{#if searching}
					<Empty title="Nothing matches">
						<button type="button" class="h-11 self-start rounded-full bg-well px-5 text-[15px] font-bold text-ink" onclick={() => (f.q = '')}>Clear</button>
					</Empty>
				{/if}
			{/each}
		{/if}
	</div>

	<!-- The pane scrolls on its own, so a long entry leaves the list where it was.
	     It reaches into the tabs' bottom padding, which would otherwise lift it
	     at the end of the page. -->
	<div class="hidden lg:sticky lg:top-0 lg:-mb-8 lg:block lg:h-dvh lg:overflow-y-auto">
		<!-- The form brings its own side padding. -->
		<div class="mx-auto flex max-w-[640px] flex-col gap-3.5 pb-8 {(f.new && wide.current) || editing ? 'px-4' : 'px-8'}">
			{#if f.new && wide.current}
				<EntryForm today={data.today} cancel={pane()} saved={(e) => show(pane('open', e.id))} heading="h2" />
			{:else if opened && editing}
				{#key opened.id}
					<EntryForm
						entry={opened}
						today={data.today}
						cancel={pane('open', opened.id)}
						saved={(e) => show(pane('open', e.id))}
						heading="h2"
					/>
				{/key}
			{:else if opened}
				{@const e = opened}
				<div class="flex justify-end gap-2 pt-3">
					<a
						href={pane('edit', e.id)}
						data-sveltekit-replacestate
						data-sveltekit-noscroll
						class="flex h-11 items-center rounded-full bg-surface px-4 text-[15px] font-bold text-ink shadow-card-sm">Edit</a
					>
					<Menu
						items={[
							{ label: e.pinned_at ? 'Unpin' : 'Pin', onclick: () => pin(e, !e.pinned_at) },
							{ label: 'Delete entry', danger: true, onclick: () => remove(e) }
						]}
					/>
				</div>
				<FormError message={error} />
				<EntryView entry={e} heading="h2" />
			{:else if total}
				<div class="pt-3">
					<Empty title="No entry open" text="Entries you write open here. A session's journal opens in History.">
						<Button href={pane('new')}>New entry</Button>
					</Empty>
				</div>
			{/if}
		</div>
	</div>
</div>

<style>
	/* The open entry's row, drawn as a well that reaches past the card's
	   dividers. */
	.picked {
		margin-inline: -0.625rem;
		padding-inline: 0.625rem;
		border-radius: 1rem;
		background: var(--well);
		box-shadow: none;
	}
	.picked + :global(li) {
		box-shadow: none;
	}
</style>
