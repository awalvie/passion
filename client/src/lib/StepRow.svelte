<script lang="ts">
	import { countLabels, kindOf, setByLadder } from './exercise';
	import RowActions from './RowActions.svelte';
	import { rowCounts, stepMeta, whole, type Step } from './template';

	// One exercise in a section or a choice. A tap opens it; Alt with an arrow
	// moves it, as Move up and Move down in its menu do. From lg its sets, reps
	// and set rest are fields in the row.
	let {
		step = $bindable(),
		selected,
		onselect,
		index,
		count,
		move,
		remove
	}: {
		step: Step;
		selected: boolean;
		onselect: () => void;
		index: number;
		count: number;
		move: (by: number) => void;
		remove: () => void;
	} = $props();

	const inRow = $derived(rowCounts.filter((c) => kindOf(step.kind).counts.includes(c) && !setByLadder(step, c)));

	function key(e: KeyboardEvent) {
		if (!e.altKey) return;
		const by = e.key === 'ArrowUp' ? -1 : e.key === 'ArrowDown' ? 1 : 0;
		if (!by || index + by < 0 || index + by >= count) return;
		e.preventDefault();
		move(by);
	}
</script>

<div class="flex min-h-16 items-center gap-2 {selected ? '-mx-2 rounded-2xl bg-well pl-2 outline-2 -outline-offset-2 outline-ink' : ''}">
	<button type="button" class="min-w-0 flex-1 py-3 text-left" aria-pressed={selected} onclick={onselect} onkeydown={key}>
		<span class="block truncate text-[15px] font-bold">{step.name}</span>
		<span class="block truncate text-xs font-semibold text-ink-2">{stepMeta(step)}</span>
	</button>
	{#if inRow.length}
		<span class="hidden shrink-0 items-center gap-1.5 text-xs font-semibold text-ink-2 lg:flex">
			{#each inRow as c, n (c)}
				{#if c === 'reps' && n}<span aria-hidden="true">×</span>{/if}
				<input
					type="number"
					min="0"
					step="1"
					class="input digits h-11 px-1 text-center {c === 'set_rest_seconds' ? 'w-16' : 'w-12'} {selected ? 'bg-surface' : ''}"
					aria-label="{countLabels[c]} for {step.name}"
					bind:value={step[c]}
					onchange={() => (step[c] = whole(step[c]))}
				/>
				{#if c === 'set_rest_seconds'}<span>s rest</span>{/if}
			{/each}
		</span>
	{/if}
	<RowActions label={step.name} {index} {count} {move} {remove} />
</div>
