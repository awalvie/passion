<script lang="ts">
	import { notice } from '$lib/notice.svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { describe, request } from '$lib/api';
	import { fieldLabel, kindOf, toBody, toDraft, type Count, type Exercise } from '$lib/exercise';
	import ExerciseForm from '$lib/ExerciseForm.svelte';
	import FormBar from '$lib/FormBar.svelte';
	import { guardUnsaved } from '$lib/unsaved';
	import { untrack } from 'svelte';
	import { MediaQuery } from 'svelte/reactivity';

	let { data } = $props();

	let draft = $state(toDraft());
	let error = $state('');
	let busy = $state(false);

	const clean = untrack(() => JSON.stringify(draft));
	let leaving = false;
	guardUnsaved(() => !leaving && JSON.stringify(draft) !== clean);

	const shown = (c: Count) => kindOf(draft.kind).counts.includes(c);

	// From lg the new exercise opens beside the list, and Cancel goes back to
	// the one that was open, if it is still in the list.
	const wide = new MediaQuery('min-width: 64rem');
	const open = untrack(() => data.exercises.find((e) => e.id === page.url.searchParams.get('open'))?.id);

	async function save(event: SubmitEvent) {
		event.preventDefault();
		error = '';
		busy = true;
		try {
			const made = await request<Exercise>('POST', '/api/v1/exercises', toBody(draft, shown));
			leaving = true;
			if (wide.current) await goto(`/exercises/${made.id}`, { replaceState: true, noScroll: true, invalidateAll: true });
			else await goto('/exercises');
			notice.text = 'Exercise saved';
		} catch (e) {
			error = describe(e, fieldLabel);
		} finally {
			busy = false;
		}
	}
</script>

<svelte:head><title>New exercise</title></svelte:head>

<!-- From lg the Library's pane brings the side padding. -->
<form class="mx-auto flex w-full max-w-xl flex-col gap-3.5 px-4 pb-6 lg:px-0" onsubmit={save}>
	<FormBar
		title="New exercise"
		heading={wide.current ? 'h2' : 'h1'}
		cancel={wide.current && open ? `/exercises/${open}` : '/exercises'}
		replace={wide.current}
		{busy}
		{error}
		class="-mx-4"
	/>
	<p class="px-1 text-[15px] font-semibold text-ink-2">Once it is saved, you can add it to any session.</p>
	<div class="rounded-3xl bg-surface p-[18px] shadow-card">
		<ExerciseForm bind:draft {shown} sources={data.sources} />
	</div>
</form>
