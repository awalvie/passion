<script lang="ts">
	import Checkbox from './Checkbox.svelte';
	import DurationInput from '$lib/DurationInput.svelte';
	import { allCounts, countLabels, kindOf, setByLadder, type Count } from '$lib/exercise';
	import { whole, type Step } from '$lib/template';

	// The fields of one exercise in a session, but for those in skip, which
	// its row shows.
	let { step = $bindable(), id, skip = [] }: { step: Step; id: string; skip?: Count[] } = $props();

	// A step never changes type here, so the numbers on screen are its type's,
	// plus any it held when it opened.
	const held = new Set(allCounts.filter((c) => step[c] != null));
	const shown = allCounts.filter((c) => (kindOf(step.kind).counts.includes(c) || held.has(c)) && !setByLadder(step, c));
	const counts = shown.filter((c) => c !== 'duration_seconds' && !(skip.includes(c) && kindOf(step.kind).counts.includes(c)));
</script>

<div class="flex flex-col gap-3">
	{#if counts.length}
		<div class="grid grid-cols-3 gap-2.5">
			{#each counts as c (c)}
				<div>
					<label class="block text-xs font-semibold text-ink-2" for="{id}-{c}">{countLabels[c]}</label>
					<input id="{id}-{c}" type="number" min="0" step="1" class="mt-1.5 min-h-11 input py-2 text-center" bind:value={step[c]} onchange={() => (step[c] = whole(step[c]))} />
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
		<textarea id="{id}-notes" rows="3" class="mt-1.5 input" bind:value={step.notes}></textarea>
	</div>
</div>
