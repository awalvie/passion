<script lang="ts">
	import Checkbox from './Checkbox.svelte';
	import type { Snippet } from 'svelte';
	import DurationInput from '$lib/DurationInput.svelte';
	import Icon from '$lib/Icon.svelte';
	import { allCounts, countLabels, kindOf } from '$lib/exercise';
	import { stepMeta, type Step } from '$lib/template';

	let { step = $bindable(), id, actions }: { step: Step; id: string; actions: Snippet } = $props();

	// A step never changes type here, so the numbers on screen are its type's,
	// plus any it held when the page opened.
	const held = new Set(allCounts.filter((c) => step[c] != null));
	const shown = allCounts.filter((c) => kindOf(step.kind).counts.includes(c) || held.has(c));
	const counts = shown.filter((c) => c !== 'duration_seconds');
</script>

<details class="group/step">
	<summary class="flex min-h-16 cursor-pointer list-none items-center gap-3 py-3 [&::-webkit-details-marker]:hidden">
		<span class="text-ink-3 transition-transform group-open/step:rotate-90"><Icon name="chevron-right" /></span>
		<div class="min-w-0 flex-1">
			<div class="truncate text-[15px] font-bold">{step.name}</div>
			<div class="truncate text-xs font-semibold text-ink-2">{stepMeta(step)}</div>
		</div>
		{@render actions()}
	</summary>

	<div class="flex flex-col gap-3 pb-4">
		{#if counts.length}
			<div class="grid grid-cols-3 gap-2.5">
				{#each counts as c (c)}
					<div>
						<label class="block text-xs font-semibold text-ink-2" for="{id}-{c}">{countLabels[c]}</label>
						<input
							id="{id}-{c}"
							type="number"
							min="0"
							step="1"
							class="mt-1.5 min-h-10 w-full input py-2 text-center"
							bind:value={step[c]}
						/>
					</div>
				{/each}
			</div>
		{/if}
		{#if shown.includes('duration_seconds')}
			<DurationInput bind:seconds={step.duration_seconds} id="{id}-duration" small />
		{/if}
		{#if shown.length}
			<label class="flex min-h-11 cursor-pointer items-center gap-2.5 text-[15px] font-semibold">
				<Checkbox bind:checked={step.per_side} />
				Per side
			</label>
		{/if}
		<div>
			<label class="block text-xs font-semibold text-ink-2" for="{id}-notes">Notes</label>
			<textarea id="{id}-notes" rows="3" class="mt-1.5 w-full input" bind:value={step.notes}></textarea>
		</div>
	</div>
</details>
