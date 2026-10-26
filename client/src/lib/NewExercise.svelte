<script lang="ts">
	import { untrack } from 'svelte';
	import { describe, request } from './api';
	import Button from './Button.svelte';
	import { fieldLabel, kindOf, toBody, toDraft, type Count, type Exercise } from './exercise';
	import ExerciseForm from './ExerciseForm.svelte';
	import FormError from './FormError.svelte';
	import Sheet from './Sheet.svelte';

	// Makes an exercise for the library without leaving the builder. where
	// names what it is added to once saved.
	let {
		open = $bindable(false),
		name,
		where,
		sources,
		onsaved
	}: { open?: boolean; name: string; where: string; sources: string[]; onsaved: (e: Exercise) => void } = $props();

	let draft = $state(toDraft());
	let error = $state('');
	let busy = $state(false);

	$effect(() => {
		if (open)
			untrack(() => {
				draft = { ...toDraft(), name };
				error = '';
			});
	});

	const shown = (c: Count) => kindOf(draft.kind).counts.includes(c);

	async function save() {
		error = '';
		busy = true;
		try {
			const made = await request<Exercise>('POST', '/api/v1/exercises', toBody(draft, shown));
			open = false;
			onsaved(made);
		} catch (e) {
			error = describe(e, fieldLabel);
		} finally {
			busy = false;
		}
	}
</script>

<Sheet bind:open eyebrow="Adds to {where}" title="New exercise">
	<div class="flex flex-col gap-3.5">
		<div class="rounded-3xl bg-surface p-[18px] shadow-card">
			<ExerciseForm bind:draft {shown} {sources} />
		</div>
		<FormError message={error} />
	</div>
	{#snippet footer()}
		<div class="grid grid-cols-2 items-center gap-2">
			<Button variant="secondary" onclick={() => (open = false)}>Cancel</Button>
			<Button disabled={busy || !draft.name.trim()} onclick={save}>{busy ? 'Saving…' : 'Save'}</Button>
		</div>
	{/snippet}
</Sheet>
