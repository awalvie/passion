<script lang="ts">
	import type { Snippet } from 'svelte';
	import type { Exercise } from '$lib/exercise';
	import ExercisePicker from '$lib/ExercisePicker.svelte';
	import RowActions from '$lib/RowActions.svelte';
	import StepEditor from '$lib/StepEditor.svelte';
	import { choiceMeta, moveEntry, toStep, type Choice } from '$lib/template';

	let {
		choice = $bindable(),
		id,
		library,
		actions
	}: { choice: Choice; id: string; library: Exercise[]; actions: Snippet } = $props();
</script>

<details class="card-muted overflow-hidden passion-disclosure">
	<summary class="cursor-pointer list-none flex items-center justify-between gap-3 px-3 py-2.5">
		<div class="min-w-0 flex-1">
			<div class="text-sm font-medium truncate">{choice.name}</div>
			<div class="text-xs muted truncate">{choiceMeta(choice)}</div>
		</div>
		{@render actions()}
	</summary>

	<div class="px-3 pb-3 space-y-3">
		<div class="grid gap-2 grid-cols-3">
			<div class="col-span-2">
				<label class="text-xs font-medium" for="{id}-name">Name</label>
				<input id="{id}-name" class="mt-1 w-full input text-xs" maxlength="200" required bind:value={choice.name} />
			</div>
			<div>
				<label class="text-xs font-medium" for="{id}-pick">Pick at least</label>
				<input
					id="{id}-pick"
					type="number"
					min="0"
					max={choice.options.length}
					step="1"
					class="mt-1 w-full input text-xs"
					required
					bind:value={choice.pick}
				/>
			</div>
		</div>
		<p class="text-[10px] muted -mt-2">0 makes the whole choice optional.</p>
		<div>
			<label class="text-xs font-medium" for="{id}-notes">Notes</label>
			<textarea id="{id}-notes" rows="2" class="mt-1 w-full input text-xs" bind:value={choice.notes}></textarea>
		</div>

		<div class="space-y-2">
			{#each choice.options as option, k (option)}
				{#snippet optionActions()}
					<RowActions
						label={option.name}
						index={k}
						count={choice.options.length}
						move={(by) => moveEntry(choice.options, k, by)}
						remove={() => choice.options.splice(k, 1)}
					/>
				{/snippet}
				<StepEditor bind:step={choice.options[k]} id="{id}-opt{k}" actions={optionActions} />
			{:else}
				<div class="p-2 text-xs muted">No options yet. A choice needs at least one.</div>
			{/each}
		</div>

		<ExercisePicker exercises={library} id="{id}-add" pick={(e) => choice.options.push(toStep(e))} />
	</div>
</details>
