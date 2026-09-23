<script lang="ts">
	import type { Snippet } from 'svelte';
	import DurationInput from '$lib/DurationInput.svelte';
	import { allCounts, countLabels, kindOf } from '$lib/exercise';
	import { stepMeta, type Step } from '$lib/template';

	let { step = $bindable(), id, actions }: { step: Step; id: string; actions: Snippet } = $props();

	// A step never changes type here, so the numbers on screen are its type's,
	// plus any it held when the page opened.
	const held = new Set(allCounts.filter((c) => step[c] != null));
	const shown = allCounts.filter((c) => kindOf(step.kind).counts.includes(c) || held.has(c));
	const counts = shown.filter((c) => c !== 'duration_seconds');
</script>

<details class="card-muted overflow-hidden passion-disclosure">
	<summary class="cursor-pointer list-none flex items-center justify-between gap-3 px-3 py-2.5">
		<div class="min-w-0 flex-1">
			<div class="text-sm font-medium truncate">{step.name}</div>
			<div class="text-xs muted truncate">{stepMeta(step)}</div>
		</div>
		{@render actions()}
	</summary>

	<div class="px-3 pb-3 space-y-3">
		{#if counts.length}
			<div class="grid gap-2 grid-cols-3">
				{#each counts as c (c)}
					<div>
						<label class="text-xs font-medium" for="{id}-{c}">{countLabels[c]}</label>
						<input
							id="{id}-{c}"
							type="number"
							min="0"
							step="1"
							class="mt-1 w-full input text-xs"
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
			<label class="flex items-center gap-2 text-xs font-medium cursor-pointer">
				<input type="checkbox" class="rounded" bind:checked={step.per_side} />
				Per side
			</label>
		{/if}
		<div>
			<label class="text-xs font-medium" for="{id}-notes">Notes</label>
			<textarea id="{id}-notes" rows="3" class="mt-1 w-full input text-xs" bind:value={step.notes}></textarea>
		</div>
	</div>
</details>
