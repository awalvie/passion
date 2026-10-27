<script lang="ts">
	import { notice } from '$lib/notice.svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { describe, request } from '$lib/api';
	import { allCounts, fieldLabel, kindOf, setByLadder, summary, toBody, toDraft, type Count, type Exercise } from '$lib/exercise';
	import ExerciseForm from '$lib/ExerciseForm.svelte';
	import ExerciseHistory from '$lib/ExerciseHistory.svelte';
	import FormBar from '$lib/FormBar.svelte';
	import FormError from '$lib/FormError.svelte';
	import Icon from '$lib/Icon.svelte';
	import Menu from '$lib/Menu.svelte';
	import NavBar from '$lib/NavBar.svelte';
	import Section from '$lib/Section.svelte';
	import { guardUnsaved } from '$lib/unsaved';
	import { onMount, untrack } from 'svelte';
	import { MediaQuery } from 'svelte/reactivity';

	let { data } = $props();

	// From lg a saved exercise stays open beside the list.
	const wide = new MediaQuery('min-width: 64rem');
	// +layout.svelte builds a new page for another exercise or a save, so the
	// first value is the only one.
	const saved = untrack(() => data.exercise);
	let draft = $state(toDraft(saved));
	let error = $state('');
	let busy = $state(false);

	const clean = untrack(() => JSON.stringify(draft));
	const dirty = () => JSON.stringify(draft) !== clean;
	let leaving = false;
	guardUnsaved(() => !leaving && dirty());

	// A copy just made opens with its name picked out to type over.
	onMount(() => {
		const name = page.url.searchParams.has('rename') ? (document.getElementById('ex-name') as HTMLInputElement | null) : null;
		name?.focus();
		name?.select();
	});

	const locked = saved.shipped
		? 'The app ships this exercise, so it cannot be changed. Duplicate it to make your own.'
		: saved.retired_at
			? 'You retired this exercise. Sessions that use it keep their copy.'
			: '';

	// A number the exercise already holds stays on screen whatever its type, so
	// a save never drops a value the page did not show.
	const held = new Set(allCounts.filter((c) => saved[c] != null));
	const shown = (c: Count) => (kindOf(draft.kind).counts.includes(c) || held.has(c)) && !setByLadder(draft, c);

	const thumb = saved.media[0];
	// The page shows the newest sessions; the full list is one tap away.
	const rows = 3;

	async function save(event: SubmitEvent) {
		event.preventDefault();
		error = '';
		busy = true;
		try {
			await request('PUT', `/api/v1/exercises/${saved.id}`, toBody(draft, shown));
			leaving = true;
			if (wide.current) {
				const url = new URL(page.url);
				url.searchParams.delete('rename');
				await goto(url, { replaceState: true, noScroll: true, invalidateAll: true, state: { saved: Date.now() } });
			} else await goto(from ?? '/exercises');
			notice.text = 'Exercise saved';
		} catch (e) {
			error = describe(e, fieldLabel);
		} finally {
			busy = false;
		}
	}

	// The copy is your own, so it can be changed even when this one is built in.
	async function duplicate() {
		if (busy || (dirty() && !confirm('Discard your unsaved changes?'))) return;
		error = '';
		busy = true;
		try {
			const copy = await request<Exercise>(
				'POST',
				'/api/v1/exercises',
				toBody({ ...toDraft(saved), name: `${saved.name} (copy)` }, (c) => kindOf(saved.kind).counts.includes(c) || held.has(c))
			);
			leaving = true;
			await goto(`/exercises/${copy.id}?rename`);
		} catch (e) {
			error = describe(e, fieldLabel);
		} finally {
			busy = false;
		}
	}

	async function retire() {
		if (busy) return;
		if (!confirm(`Retire “${saved.name}”? It leaves your library. Sessions that use it keep their copy.`))
			return;
		error = '';
		busy = true;
		try {
			await request('POST', `/api/v1/exercises/${saved.id}/retire`);
			leaving = true;
			await goto(from ?? '/exercises');
		} catch (e) {
			error = describe(e, fieldLabel);
		} finally {
			busy = false;
		}
	}

	// A row on a session page opens its exercise and leads back there.
	const from = $derived(page.url.searchParams.get('from')?.match(/^\/templates\/[\w-]+(\?tab=exercises)?$/)?.[0]);
</script>

<svelte:head><title>{saved.name}</title></svelte:head>

{#snippet menu()}
	<Menu items={[{ label: 'Duplicate', onclick: duplicate }, ...(locked ? [] : [{ label: 'Retire', danger: true, onclick: retire }])]} />
{/snippet}

<!-- On a phone one you can change has the form bar, and one you cannot has a
     nav bar. Beside the Library's list Save sits by the title, so a built-in
     exercise and your own start at the same height. -->
{#if !wide.current && !locked}
	<FormBar title="Edit exercise" heading="p" cancel={from ?? '/exercises'} form="exercise-form" {busy} {error} />
{:else if !wide.current}
	<NavBar heading={false} back={from ? { href: from, label: 'Session' } : { href: '/exercises', label: 'Exercises' }} actions={menu} />
{/if}

<div class="grid gap-3.5 px-4 pt-2 lg:pt-3 [&>*]:min-w-0">
	<div class="order-first flex flex-col gap-3.5">
		{#if wide.current && from}
			<div class="flex">
				<a
					href={from}
					class="inline-flex h-11 items-center gap-0.5 rounded-full bg-surface pr-4 pl-2.5 text-[15px] font-bold text-ink shadow-card-sm"
				>
					<Icon name="chevron-left" size="1.25rem" stroke={2.4} />Session
				</a>
			</div>
		{/if}
		<header class="flex items-start gap-3">
			<div class="min-w-0 flex-1 px-1">
				<svelte:element this={wide.current ? 'h2' : 'h1'} class="page-title break-words">{saved.name}</svelte:element>
				<p class="mt-1 text-[15px] font-semibold text-ink-2">
					{locked || 'Changes here do not affect sessions that already use it.'}
				</p>
			</div>
			{#if wide.current && !locked}
				{#if dirty()}
					<button
						type="button"
						class="flex h-11 items-center rounded-full bg-surface px-4 text-[15px] font-bold text-ink shadow-card-sm"
						onclick={() => (draft = toDraft(saved))}>Cancel</button
					>
				{/if}
				<button
					type="submit"
					form="exercise-form"
					class="flex h-11 items-center rounded-full bg-tint px-5 text-[15px] font-bold text-on-tint shadow-tint disabled:opacity-50"
					disabled={busy}>{busy ? 'Saving…' : 'Save'}</button
				>
			{/if}
			{#if wide.current || !locked}{@render menu()}{/if}
		</header>
		{#if locked || wide.current}<FormError message={error} />{/if}
	</div>

	<div class="flex flex-col gap-3.5">
		<form id="exercise-form" class="rounded-3xl bg-surface p-[18px] shadow-card" onsubmit={save}>
			<ExerciseForm bind:draft {shown} sources={data.sources} disabled={Boolean(locked)} />
		</form>

		{#if data.sessions.length}
			<Section title="History">
				{#snippet aside()}
					{#if data.sessions.length > rows}
						<a
							href="/history/exercise/{saved.id}?from=/exercises/{saved.id}"
							class="-my-3 flex h-11 items-center gap-0.5 text-xs font-bold text-ink"
							>All {data.sessions.length} sessions<Icon name="chevron-right" size="14px" /></a
						>
					{/if}
				{/snippet}
				<ExerciseHistory sessions={data.sessions} scales={data.scales} sides={saved.per_side ? 2 : 1} {rows} href={(s) => `/history/${s.run}`} />
			</Section>
		{/if}
	</div>

	<!-- From lg an exercise you can change puts its form first, as the picture
	     would push the form below the fold. A locked one keeps it on top, as it
	     holds the media. -->
	<aside class="order-first {locked ? '' : 'lg:order-last'}">
		<div class="rounded-3xl bg-surface p-2.5 shadow-card">
			<div class="flex aspect-[16/10] items-center justify-center overflow-hidden rounded-[18px] bg-well">
				{#if thumb?.thumb_url}
					{#if thumb.url}
						<a href={thumb.url} target="_blank" rel="noopener" title="Open video" class="size-full">
							<img src={thumb.thumb_url} alt="Exercise thumbnail" class="size-full object-cover" />
						</a>
					{:else}
						<img src={thumb.thumb_url} alt="Exercise thumbnail" class="size-full object-cover" />
					{/if}
				{:else}
					<div class="flex flex-col items-center gap-2 text-xs font-semibold text-ink-2">
						<Icon name="image" size="2rem" />
						<span>{thumb ? 'No thumbnail' : 'No media added'}</span>
					</div>
				{/if}
			</div>
			<div class="px-2 pt-2.5 pb-1.5">
				<div class="text-xs font-semibold text-ink-2">Preview</div>
				<div class="mt-0.5 text-[15px] font-bold">{saved.name}</div>
				<div class="mt-0.5 text-xs font-semibold text-ink-2">{summary(saved) || kindOf(saved.kind).label}</div>
			</div>
		</div>
	</aside>
</div>
