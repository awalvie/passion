<script lang="ts">
	import type { Snippet } from 'svelte';
	import type { Exercise } from '$lib/exercise';
	import Icon from '$lib/Icon.svelte';
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

<details class="group/choice">
	<summary class="flex min-h-16 cursor-pointer list-none items-center gap-3 py-3 [&::-webkit-details-marker]:hidden">
		<span class="text-ink-3 transition-transform group-open/choice:rotate-90"><Icon name="chevron-right" /></span>
		<div class="min-w-0 flex-1">
			<div class="truncate text-[15px] font-bold">{choice.name}</div>
			<div class="truncate text-xs font-semibold text-ink-2">{choiceMeta(choice)}</div>
		</div>
		{@render actions()}
	</summary>

	<div class="flex flex-col gap-3 pb-4">
		<div class="grid grid-cols-3 gap-2.5">
			<div class="col-span-2">
				<label class="block text-xs font-semibold text-ink-2" for="{id}-name">Name</label>
				<input id="{id}-name" class="mt-1.5 w-full input" maxlength="200" required bind:value={choice.name} />
			</div>
			<div>
				<label class="block text-xs font-semibold text-ink-2" for="{id}-pick">Pick at least</label>
				<input
					id="{id}-pick"
					type="number"
					min="0"
					max={choice.options.length}
					step="1"
					class="mt-1.5 w-full input text-center"
					required
					bind:value={choice.pick}
				/>
			</div>
		</div>
		<p class="-mt-1 text-xs font-semibold text-ink-3">0 makes the whole choice optional.</p>
		<div>
			<label class="block text-xs font-semibold text-ink-2" for="{id}-notes">Notes</label>
			<textarea id="{id}-notes" rows="2" class="mt-1.5 w-full input" bind:value={choice.notes}></textarea>
		</div>

		<div class="rounded-2xl px-3.5 shadow-[inset_0_0_0_1px_var(--line)]">
			<div class="pt-3 text-xs font-bold tracking-wider text-ink-3 uppercase">Options</div>
			<div class="[&>:not(:first-child)]:shadow-[inset_0_1px_0_var(--line)]">
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
					<div class="py-3 text-xs font-semibold text-ink-2">No options yet. A choice needs at least one.</div>
				{/each}
			</div>
		</div>

		<ExercisePicker exercises={library} id="{id}-add" pick={(e) => choice.options.push(toStep(e))} />
	</div>
</details>
