<script lang="ts">
	import type { Snippet } from 'svelte';
	import ChoiceEditor from '$lib/ChoiceEditor.svelte';
	import type { Exercise } from '$lib/exercise';
	import ExercisePicker from '$lib/ExercisePicker.svelte';
	import RowActions from '$lib/RowActions.svelte';
	import StepEditor from '$lib/StepEditor.svelte';
	import { moveEntry, toStep, type Section } from '$lib/template';

	let {
		section = $bindable(),
		id,
		open = false,
		library,
		actions
	}: {
		section: Section;
		id: string;
		open?: boolean;
		library: Exercise[];
		actions: Snippet;
	} = $props();

	let newChoice = $state('');

	// A new choice opens empty; it needs an option before the template saves.
	function addChoice() {
		const name = newChoice.trim();
		if (!name) return;
		section.items.push({ choice: { name, notes: null, pick: 1, options: [] } });
		newChoice = '';
	}
</script>

<details class="card overflow-hidden passion-disclosure" {open}>
	<summary class="cursor-pointer list-none flex items-center gap-2 px-3 py-3 sm:px-4">
		<div class="flex-1 min-w-0">
			<div class="text-sm font-semibold truncate">{section.name || 'Untitled section'}</div>
			<div class="text-xs muted mt-0.5">
				{section.items.length} exercise{section.items.length === 1 ? '' : 's'}
			</div>
		</div>
		{@render actions()}
	</summary>

	<div class="px-4 pb-4 space-y-3 sm:px-5 sm:pb-5">
		<div>
			<label class="text-xs font-medium" for="{id}-name">Name</label>
			<input id="{id}-name" class="mt-1 w-full input text-sm" maxlength="200" required bind:value={section.name} />
		</div>
		<div>
			<label class="text-xs font-medium" for="{id}-notes">Notes</label>
			<textarea id="{id}-notes" rows="2" class="mt-1 w-full input text-sm" bind:value={section.notes}></textarea>
		</div>

		<div class="space-y-2">
			{#each section.items as item, j (item)}
				{#snippet itemActions()}
					<RowActions
						label={item.step?.name ?? item.choice?.name ?? 'item'}
						index={j}
						count={section.items.length}
						move={(by) => moveEntry(section.items, j, by)}
						remove={() => section.items.splice(j, 1)}
					/>
				{/snippet}
				{#if item.step}
					<StepEditor bind:step={item.step} id="{id}-item{j}" actions={itemActions} />
				{:else}
					<ChoiceEditor bind:choice={item.choice} id="{id}-item{j}" {library} actions={itemActions} />
				{/if}
			{:else}
				<div class="card-muted p-3 text-xs muted">No exercises yet.</div>
			{/each}
		</div>

		<ExercisePicker exercises={library} id="{id}-add" pick={(e) => section.items.push({ step: toStep(e) })} />

		<div class="grid gap-2 grid-cols-[1fr_auto] items-end">
			<div>
				<label class="text-xs font-medium" for="{id}-choice">Add choice</label>
				<input
					id="{id}-choice"
					class="mt-1 w-full input text-sm"
					placeholder="e.g. Drills, Stretches"
					bind:value={newChoice}
					onkeydown={(e) => {
						if (e.key === 'Enter') {
							e.preventDefault();
							addChoice();
						}
					}}
				/>
			</div>
			<button type="button" class="rounded-md btn-ghost px-3 py-2 text-sm font-medium" onclick={addChoice}>
				+ Add choice
			</button>
		</div>
	</div>
</details>
