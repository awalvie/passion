<script lang="ts">
	import type { Snippet } from 'svelte';
	import Icon from '$lib/Icon.svelte';
	import StepRow from '$lib/StepRow.svelte';
	import { choiceMeta, moveEntry, whole, type Choice, type Step } from '$lib/template';
	import { plural } from '$lib/text';

	// A choice in a section: its name opens it, and its options are rows.
	let {
		choice = $bindable(),
		id,
		selected,
		onselect,
		onadd,
		actions
	}: {
		choice: Choice;
		id: string;
		selected: unknown;
		onselect: (picked: { choice: Choice } | { step: Step }) => void;
		onadd: () => void;
		actions: Snippet;
	} = $props();
</script>

<div class="flex flex-col pb-3">
	<div class="flex min-h-16 items-center gap-2 {selected === choice ? '-mx-2 rounded-2xl bg-well pl-2 outline-2 -outline-offset-2 outline-ink' : ''}">
		<button type="button" class="min-w-0 flex-1 py-3 text-left" aria-pressed={selected === choice} onclick={() => onselect({ choice })}>
			<span class="block truncate text-[15px] font-bold">{choice.name || 'Untitled choice'}</span>
			<span class="block truncate text-xs font-semibold text-ink-2">{choiceMeta(choice) || plural(choice.options.length, 'option')}</span>
		</button>
		{@render actions()}
	</div>
	<div class="rounded-2xl px-3.5 shadow-[inset_0_0_0_1px_var(--line)]">
		<label class="flex min-h-14 items-center gap-3 text-xs font-semibold text-ink-2">
			Pick at least
			<input id="{id}-pick" type="number" min="0" max={choice.options.length} step="1" class="input h-11 w-16 px-2 text-center" bind:value={choice.pick} onchange={() => (choice.pick = whole(choice.pick) ?? choice.pick)} />
			<span class="text-ink-3">0 makes it optional</span>
		</label>
		<div class="pt-1 text-xs font-bold tracking-wider text-ink-3 uppercase">Options</div>
		<div class="[&>:not(:first-child)]:shadow-[inset_0_1px_0_var(--line)]">
			{#each choice.options as option, k (option)}
				<StepRow
					bind:step={choice.options[k]}
					selected={selected === option}
					onselect={() => onselect({ step: option })}
					index={k}
					count={choice.options.length}
					move={(by) => moveEntry(choice.options, k, by)}
					remove={() => choice.options.splice(k, 1)}
				/>
			{:else}
				<p class="py-3 text-xs font-semibold text-ink-2">No options yet. A choice needs at least one.</p>
			{/each}
			<button type="button" class="flex min-h-13 w-full items-center gap-2.5 text-[15px] font-bold" onclick={onadd}>
				<Icon name="plus" />Add option
			</button>
		</div>
	</div>
</div>
