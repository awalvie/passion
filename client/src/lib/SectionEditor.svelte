<script lang="ts">
	import type { Snippet } from 'svelte';
	import ChoiceEditor from '$lib/ChoiceEditor.svelte';
	import Icon from '$lib/Icon.svelte';
	import RowActions from '$lib/RowActions.svelte';
	import StepRow from '$lib/StepRow.svelte';
	import { choiceMeta, moveEntry, type Choice, type Section, type Step } from '$lib/template';
	import { plural } from '$lib/text';

	// One section of the session as a card. target marks the section the
	// library adds to.
	let {
		section = $bindable(),
		id,
		number,
		open = false,
		target = false,
		selected,
		onselect,
		onadd,
		actions
	}: {
		section: Section;
		id: string;
		number: number;
		open?: boolean;
		target?: boolean;
		selected: unknown;
		onselect: (picked: { choice: Choice } | { step: Step }) => void;
		onadd: (to: { section: Section } | { choice: Choice }) => void;
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
	const count = $derived((only && choiceMeta(only)) || plural(section.items.length, 'exercise'));
	const names = $derived(section.items.map((i) => i.step?.name ?? i.choice?.name).join(', '));
	const quiet = 'flex min-h-13 w-full cursor-pointer list-none items-center gap-2.5 text-[15px] font-bold [&::-webkit-details-marker]:hidden';
</script>

<details class="group/section rounded-3xl bg-surface shadow-card" bind:open>
	<summary class="flex min-h-16 cursor-pointer list-none items-center gap-3 py-2 pr-2 pl-4 [&::-webkit-details-marker]:hidden">
		<span
			class="flex size-7 shrink-0 items-center justify-center rounded-[9px] font-[family-name:var(--font-digits)] text-[15px] font-bold {target
				? 'bg-ink text-ground'
				: 'bg-well'}"
		>
			{number}
		</span>
		<div class="min-w-0 flex-1">
			<div class="truncate text-base font-bold">{section.name || 'Untitled section'}</div>
			<div class="truncate text-xs font-semibold text-ink-2">{open ? count : names || count}</div>
		</div>
		{@render actions()}
		<span class="flex size-11 shrink-0 items-center justify-center text-ink-3 transition-transform group-open/section:rotate-180"><Icon name="chevron-down" /></span>
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
				{@const step = item.step}
				<StepRow
					bind:step={item.step}
					selected={selected === step}
					onselect={() => onselect({ step })}
					index={j}
					count={section.items.length}
					move={(by) => moveEntry(section.items, j, by)}
					remove={() => section.items.splice(j, 1)}
				/>
			{:else}
				{@const choice = item.choice}
				<ChoiceEditor bind:choice={item.choice} id="{id}-item{j}" {selected} {onselect} onadd={() => onadd({ choice })} actions={itemActions} />
			{/if}
		{:else}
			<p class="my-2 rounded-2xl bg-well py-4 text-center text-[15px] font-semibold text-ink-2">No exercises yet.</p>
		{/each}

		<button type="button" class={quiet} onclick={() => onadd({ section })}><Icon name="plus" />Add exercise</button>

		<details>
			<summary class={quiet}><Icon name="plus" />Add choice</summary>
			<div class="grid grid-cols-[1fr_auto] items-end gap-2 pb-4">
				<div>
					<label class="block text-xs font-semibold text-ink-2" for="{id}-choice">Name</label>
					<input
						id="{id}-choice"
						class="mt-1.5 input"
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
				<button type="button" class="flex h-12 items-center gap-1.5 rounded-full bg-well px-4 text-[15px] font-bold text-ink active:opacity-70" onclick={addChoice}>
					Add
				</button>
			</div>
		</details>
	</div>
</details>
