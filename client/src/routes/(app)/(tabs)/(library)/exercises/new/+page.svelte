<script lang="ts">
	import { notice } from '$lib/notice.svelte';
	import { goto } from '$app/navigation';
	import { describe, request } from '$lib/api';
	import Button from '$lib/Button.svelte';
	import { fieldLabel, kindOf, toBody, toDraft, type Count } from '$lib/exercise';
	import ExerciseForm from '$lib/ExerciseForm.svelte';
	import FormError from '$lib/FormError.svelte';

	let { data } = $props();

	let draft = $state(toDraft());
	let error = $state('');
	let busy = $state(false);

	const shown = (c: Count) => kindOf(draft.kind).counts.includes(c);

	async function save(event: SubmitEvent) {
		event.preventDefault();
		error = '';
		busy = true;
		try {
			await request('POST', '/api/v1/exercises', toBody(draft, shown));
			await goto('/exercises');
			notice.text = 'Exercise saved';
		} catch (e) {
			error = describe(e, fieldLabel);
		} finally {
			busy = false;
		}
	}
</script>

<svelte:head><title>New exercise</title></svelte:head>

<div class="mx-auto flex w-full max-w-xl flex-col gap-3.5 pt-2">
	<header class="px-1">
		<h1 class="text-[32px] leading-[1.1] font-extrabold tracking-[-0.02em]">New exercise</h1>
		<p class="mt-1 text-[15px] font-semibold text-ink-2">Once it is saved, you can add it to any session template.</p>
	</header>

	<form class="flex flex-col gap-3.5" onsubmit={save}>
		<div class="rounded-3xl bg-surface p-[18px] shadow-card">
			<ExerciseForm bind:draft {shown} sources={data.sources} />
		</div>
		<FormError message={error} />
		<div
			class="sticky bottom-[calc(var(--above-bar)-0.75rem)] z-20 -mx-4 grid grid-cols-[auto_1fr] gap-2.5 bg-ground/85 px-4 py-3 backdrop-blur-md"
		>
			<a
				class="flex h-14 items-center rounded-full bg-surface px-6 text-[15px] font-bold text-ink shadow-card"
				href="/exercises">Cancel</a
			>
			<Button type="submit" disabled={busy}>{busy ? 'Saving…' : 'Save'}</Button>
		</div>
	</form>
</div>
