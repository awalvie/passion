<script lang="ts">
	import { goto } from '$app/navigation';
	import { describe, request } from '$lib/api';
	import { allCounts, fieldLabel, kindOf, summary, toBody, toDraft, type Count } from '$lib/exercise';
	import ExerciseForm from '$lib/ExerciseForm.svelte';
	import FormError from '$lib/FormError.svelte';
	import Icon from '$lib/Icon.svelte';
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
			? 'You retired this exercise. Sessions that use it keep their copy.'
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
		if (!confirm(`Retire “${saved.name}”? It leaves your library. Sessions that use it keep their copy.`))
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

<div class="lib-edit-grid">
	<div class="card card-pad">
		<div class="flex items-center justify-between gap-3">
			<div>
				<h1 class="text-xl font-bold">{locked ? saved.name : 'Edit exercise'}</h1>
				<p class="mt-1 text-xs muted">
					{locked || 'Changes here do not change sessions that already use it.'}
				</p>
			</div>
			<a
				class="rounded-md btn-ghost px-3 py-2 text-sm shrink-0"
				href="/exercises"
				aria-label="Back to exercise library"
			>
				<Icon name="arrow-left" size="0.875rem" />
			</a>
		</div>

		<form class="mt-5" onsubmit={save}>
			<ExerciseForm bind:draft {shown} sources={data.sources} disabled={Boolean(locked)} />
			{#if !locked}
				<FormError message={error} />
				<div class="lib-edit-sticky-bar">
					<button class="rounded-md btn-primary px-4 py-2 text-sm font-medium" type="submit" disabled={busy}>
						{busy ? 'Saving…' : 'Save'}
					</button>
					<span class="flex-1"></span>
					<button
						type="button"
						class="rounded-md btn-ghost px-4 py-2 text-sm inline-flex items-center"
						style="color:var(--destructive)"
						title="Retire"
						disabled={busy}
						onclick={retire}
					>
						<Icon name="archive" size="0.875rem" />
						<span class="ml-1.5 hidden sm:inline">Retire</span>
					</button>
					<a class="text-sm muted hover:underline" href="/exercises">Back</a>
				</div>
			{/if}
		</form>
	</div>

	<!-- On a phone the preview goes above the form (see .lib-edit-side). -->
	<div class="lib-edit-side space-y-4">
		<div class="card card-pad">
			<div class="text-xs font-semibold muted uppercase tracking-widest mb-3">Preview</div>
			<div class="lib-edit-media-preview">
				{#if thumb?.thumb_url}
					{#if thumb.url}
						<a href={thumb.url} target="_blank" rel="noopener" title="Open video">
							<img src={thumb.thumb_url} alt="Exercise thumbnail" />
						</a>
					{:else}
						<img src={thumb.thumb_url} alt="Exercise thumbnail" />
					{/if}
				{:else}
					<div class="lib-edit-media-placeholder">
						<Icon name="image" size="2rem" />
						<span>{thumb ? 'No thumbnail' : 'No media added'}</span>
					</div>
				{/if}
			</div>
			<div class="mt-3 space-y-2">
				<div class="text-sm font-semibold">{saved.name}</div>
				<div class="text-xs muted">{summary(saved) || kindOf(saved.kind).label}</div>
			</div>
		</div>
	</div>
</div>
