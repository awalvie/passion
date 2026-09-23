<script lang="ts">
	import type { Snippet } from 'svelte';
	import ChoiceEditor from '$lib/ChoiceEditor.svelte';
	import RowActions from '$lib/RowActions.svelte';
	import StepEditor from '$lib/StepEditor.svelte';
	import { moveEntry, type Section } from '$lib/template';

	let {
		section = $bindable(),
		id,
		open = false,
		actions
	}: { section: Section; id: string; open?: boolean; actions: Snippet } = $props();
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
					<ChoiceEditor choice={item.choice} actions={itemActions} />
				{/if}
			{:else}
				<div class="card-muted p-3 text-xs muted">No exercises yet.</div>
			{/each}
		</div>
	</div>
</details>
