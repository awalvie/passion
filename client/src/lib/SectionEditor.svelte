<script lang="ts">
	import type { Snippet } from 'svelte';
	import ChoiceEditor from '$lib/ChoiceEditor.svelte';
	import type { Exercise } from '$lib/exercise';
	import ExercisePicker from '$lib/ExercisePicker.svelte';
	import Icon from '$lib/Icon.svelte';
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

<details class="group/section rounded-3xl bg-surface shadow-card" {open}>
	<summary class="flex min-h-16 cursor-pointer list-none items-center gap-3 px-[18px] py-3.5 [&::-webkit-details-marker]:hidden">
		<span class="text-ink-3 transition-transform group-open/section:rotate-90"><Icon name="chevron-right" /></span>
		<div class="min-w-0 flex-1">
			<div class="truncate text-xl font-bold tracking-tight">{section.name || 'Untitled section'}</div>
			<div class="mt-0.5 text-xs font-semibold text-ink-2">
				{section.items.length} exercise{section.items.length === 1 ? '' : 's'}
			</div>
		</div>
		{@render actions()}
	</summary>

	<div class="flex flex-col gap-3.5 px-[18px] pb-[18px]">
		<div>
			<label class="block text-xs font-semibold text-ink-2" for="{id}-name">Name</label>
			<input id="{id}-name" class="mt-1.5 w-full input" maxlength="200" required bind:value={section.name} />
		</div>
		<div>
			<label class="block text-xs font-semibold text-ink-2" for="{id}-notes">Notes</label>
			<textarea id="{id}-notes" rows="2" class="mt-1.5 w-full input" bind:value={section.notes}></textarea>
		</div>

		<div class="divide-y divide-line border-y border-line">
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
				<div class="py-3.5 text-[15px] font-semibold text-ink-2">No exercises yet.</div>
			{/each}
		</div>

		<ExercisePicker exercises={library} id="{id}-add" pick={(e) => section.items.push({ step: toStep(e) })} />

		<div class="grid grid-cols-[1fr_auto] items-end gap-2">
			<div>
				<label class="block text-xs font-semibold text-ink-2" for="{id}-choice">Add choice</label>
				<input
					id="{id}-choice"
					class="mt-1.5 w-full input"
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
			<button
				type="button"
				class="flex h-12 items-center gap-1.5 rounded-full bg-well px-4 text-[15px] font-bold text-ink active:opacity-70"
				onclick={addChoice}
			>
				<Icon name="plus" />
				Add choice
			</button>
		</div>
	</div>
</details>
