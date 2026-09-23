<script lang="ts">
	import Icon from '$lib/Icon.svelte';

	let {
		label,
		index,
		count,
		move,
		remove
	}: {
		label: string;
		index: number;
		count: number;
		move: (by: number) => void;
		remove: () => void;
	} = $props();

	// These sit inside a <summary>, where a click would also open or close the row.
	function run(event: MouseEvent, action: () => void) {
		event.preventDefault();
		event.stopPropagation();
		action();
	}
</script>

<div class="flex shrink-0 items-center gap-0.5">
	<button
		type="button"
		class="rounded-md btn-ghost p-1.5 inline-flex items-center justify-center disabled:opacity-40"
		title="Move up"
		aria-label="Move {label} up"
		disabled={index === 0}
		onclick={(e) => run(e, () => move(-1))}
	>
		<Icon name="arrow-up" size="0.875rem" />
	</button>
	<button
		type="button"
		class="rounded-md btn-ghost p-1.5 inline-flex items-center justify-center disabled:opacity-40"
		title="Move down"
		aria-label="Move {label} down"
		disabled={index === count - 1}
		onclick={(e) => run(e, () => move(1))}
	>
		<Icon name="arrow-down" size="0.875rem" />
	</button>
	<button
		type="button"
		class="rounded-md btn-ghost p-1.5 inline-flex items-center justify-center disabled:opacity-40"
		title="Remove"
		aria-label="Remove {label}"
		onclick={(e) => run(e, remove)}
	>
		<Icon name="trash-2" size="0.875rem" />
	</button>
</div>
