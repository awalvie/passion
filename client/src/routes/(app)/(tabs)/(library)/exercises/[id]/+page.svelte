<script lang="ts">
	import { goto } from '$app/navigation';
	import { describe, request } from '$lib/api';
	import { allCounts, fieldLabel, kindOf, summary, toBody, toDraft, type Count } from '$lib/exercise';
	import Button from '$lib/Button.svelte';
	import ExerciseForm from '$lib/ExerciseForm.svelte';
	import FormError from '$lib/FormError.svelte';
	import Icon from '$lib/Icon.svelte';
	import Menu from '$lib/Menu.svelte';
	import NavBar from '$lib/NavBar.svelte';
	import { untrack } from 'svelte';

	let { data } = $props();

	// +layout.svelte builds a new page for each id, so the first value is the only one.
	const saved = untrack(() => data.exercise);
	let draft = $state(toDraft(saved));
	let error = $state('');
	let busy = $state(false);

	const locked = saved.shipped
		? 'The app ships this exercise, so it cannot be changed.'
		: saved.retired_at
			? 'You retired this exercise. Session templates that use it keep their copy.'
			: '';

	// A number the exercise already holds stays on screen whatever its type, so
	// a save never drops a value the page did not show.
	const held = new Set(allCounts.filter((c) => saved[c] != null));
	const shown = (c: Count) => kindOf(draft.kind).counts.includes(c) || held.has(c);

	const thumb = saved.media[0];

	async function save(event: SubmitEvent) {
		event.preventDefault();
		error = '';
		busy = true;
		try {
			await request('PUT', `/api/v1/exercises/${saved.id}`, toBody(draft, shown));
			await goto('/exercises');
		} catch (e) {
			error = describe(e, fieldLabel);
		} finally {
			busy = false;
		}
	}

	async function retire() {
		if (busy) return;
		if (!confirm(`Retire “${saved.name}”? It leaves your library. Session templates that use it keep their copy.`))
			return;
		error = '';
		busy = true;
		try {
			await request('POST', `/api/v1/exercises/${saved.id}/retire`);
			await goto('/exercises');
		} catch (e) {
			error = describe(e, fieldLabel);
		} finally {
			busy = false;
		}
	}
</script>

<svelte:head><title>{saved.name}</title></svelte:head>

<NavBar title={locked ? '' : 'Edit exercise'} heading={false} back={{ href: '/exercises', label: 'Exercises' }}>
	{#snippet actions()}
		{#if !locked}
			<Menu items={[{ label: 'Retire', danger: true, onclick: retire }]} />
		{/if}
	{/snippet}
</NavBar>

<div class="mx-auto grid max-w-5xl gap-3.5 px-4 pt-2 lg:grid-cols-[minmax(0,1fr)_20rem] [&>*]:min-w-0">
	<div class="flex flex-col gap-3.5">
		<header class="px-1">
			<h1 class="text-[32px] leading-[1.1] font-extrabold tracking-[-0.02em] break-words">{saved.name}</h1>
			<p class="mt-1 text-[15px] font-semibold text-ink-2">
				{locked || 'Changes here do not affect session templates that already use it.'}
			</p>
		</header>

		<form class="flex flex-col gap-3.5" onsubmit={save}>
			<div class="rounded-3xl bg-surface p-[18px] shadow-card">
				<ExerciseForm bind:draft {shown} sources={data.sources} disabled={Boolean(locked)} />
			</div>
			{#if !locked}
				<FormError message={error} />
				<div class="sticky bottom-[var(--above-bar)] z-20">
					<Button type="submit" disabled={busy}>{busy ? 'Saving…' : 'Save'}</Button>
				</div>
			{/if}
		</form>
	</div>

	<!-- On a phone the preview goes above the form rather than below a long field list. -->
	<aside class="order-first lg:order-none">
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
