<script lang="ts">
	import type { Snippet } from 'svelte';
	import ChoiceEditor from '$lib/ChoiceEditor.svelte';
	import type { Exercise } from '$lib/exercise';
	import ExercisePicker from '$lib/ExercisePicker.svelte';
	import Icon from '$lib/Icon.svelte';
	import RowActions from '$lib/RowActions.svelte';
	import StepEditor from '$lib/StepEditor.svelte';
	import { choiceMeta, moveEntry, toStep, type Section } from '$lib/template';

	let {
		section = $bindable(),
		id,
		number,
		open = false,
		library,
		actions
	}: {
		section: Section;
		id: string;
		number: number;
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

	const only = $derived(section.items.length === 1 ? section.items[0].choice : undefined);
	const count = $derived(
		only ? choiceMeta(only) : `${section.items.length} exercise${section.items.length === 1 ? '' : 's'}`
	);
	const quiet =
		'flex min-h-13 cursor-pointer list-none items-center gap-2.5 text-[15px] font-bold [&::-webkit-details-marker]:hidden';
</script>

<details class="group/section rounded-3xl bg-surface shadow-card" {open}>
	<summary class="flex min-h-16 cursor-pointer list-none items-center gap-3 py-2 pr-2 pl-4 [&::-webkit-details-marker]:hidden">
		<span class="flex size-7 shrink-0 items-center justify-center rounded-[9px] bg-well font-[family-name:var(--font-digits)] text-[15px] font-bold">
			{number}
		</span>
		<div class="min-w-0 flex-1">
			<div class="truncate text-base font-bold">{section.name || 'Untitled section'}</div>
			<div class="truncate text-xs font-semibold text-ink-2">{count}</div>
		</div>
		{@render actions()}
	</summary>

	<div class="flex flex-col px-4 pb-1 [&>:not(:first-child)]:shadow-[inset_0_1px_0_var(--line)]">
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
		{/each}

		<details>
			<summary class={quiet}><Icon name="plus" />Add exercise</summary>
			<div class="pb-4">
				<ExercisePicker exercises={library} id="{id}-add" label="Exercise" pick={(e) => section.items.push({ step: toStep(e) })} />
			</div>
		</details>

		<details>
			<summary class={quiet}><Icon name="plus" />Add choice</summary>
			<div class="grid grid-cols-[1fr_auto] items-end gap-2 pb-4">
				<div>
					<label class="block text-xs font-semibold text-ink-2" for="{id}-choice">Name</label>
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
					Add
				</button>
			</div>
		</details>

		<details>
			<summary class="{quiet} text-ink-2"><Icon name="pencil" />Name and notes</summary>
			<div class="flex flex-col gap-3.5 pb-4">
				<div>
					<label class="block text-xs font-semibold text-ink-2" for="{id}-name">Name</label>
					<input id="{id}-name" class="mt-1.5 w-full input" maxlength="200" required bind:value={section.name} />
				</div>
				<div>
					<label class="block text-xs font-semibold text-ink-2" for="{id}-notes">Notes</label>
					<textarea id="{id}-notes" rows="2" class="mt-1.5 w-full input" bind:value={section.notes}></textarea>
				</div>
			</div>
		</details>
	</div>
</details>
