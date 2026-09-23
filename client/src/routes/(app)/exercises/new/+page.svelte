<script lang="ts">
	import { goto } from '$app/navigation';
	import { describe, request } from '$lib/api';
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
		} catch (e) {
			error = describe(e, fieldLabel);
		} finally {
			busy = false;
		}
	}
</script>

<svelte:head><title>New exercise</title></svelte:head>

<div class="max-w-xl w-full mx-auto">
	<div class="card card-pad">
		<h1 class="text-xl font-bold">New exercise</h1>
		<p class="mt-1 text-sm muted">Once it is saved, you can add it to any session template.</p>

		<form class="mt-6" onsubmit={save}>
			<ExerciseForm bind:draft {shown} sources={data.sources} />
			<FormError message={error} />
			<div class="flex items-center gap-2 pt-5">
				<button class="rounded-md btn-primary px-4 py-2 text-sm font-medium" type="submit" disabled={busy}>
					{busy ? 'Saving…' : 'Save'}
				</button>
				<a class="rounded-md btn-ghost px-4 py-2 text-sm" href="/exercises">Cancel</a>
			</div>
		</form>
	</div>
</div>
