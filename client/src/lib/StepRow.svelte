<script lang="ts">
	import RowActions from './RowActions.svelte';
	import { stepMeta, type Step } from './template';

	// One exercise in a section or a choice. A tap opens it; Alt with an arrow
	// moves it, as Move up and Move down in its menu do.
	let {
		step,
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

	function key(e: KeyboardEvent) {
		if (!e.altKey) return;
		const by = e.key === 'ArrowUp' ? -1 : e.key === 'ArrowDown' ? 1 : 0;
		if (!by || index + by < 0 || index + by >= count) return;
		e.preventDefault();
		move(by);
	}
</script>

<div class="flex min-h-16 items-center gap-2 {selected ? '-mx-2 rounded-2xl bg-well pl-2 shadow-[inset_0_0_0_2px_var(--ink)]' : ''}">
	<button type="button" class="min-w-0 flex-1 py-3 text-left" aria-pressed={selected} onclick={onselect} onkeydown={key}>
		<span class="block truncate text-[15px] font-bold">{step.name}</span>
		<span class="block truncate text-xs font-semibold text-ink-2">{stepMeta(step)}</span>
	</button>
	<RowActions label={step.name} {index} {count} {move} {remove} />
</div>
