<script lang="ts">
	import Menu, { type MenuItem } from '$lib/Menu.svelte';

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

	const items = $derived<MenuItem[]>([
		...(index > 0 ? [{ label: 'Move up', onclick: () => move(-1) }] : []),
		...(index < count - 1 ? [{ label: 'Move down', onclick: () => move(1) }] : []),
		{ label: 'Remove', danger: true, onclick: remove }
	]);

	// The menu sits inside a <summary>, where a click (Enter and Space on a button are clicks too)
	// would also open or close the row.
	const hold = (e: Event) => e.preventDefault();
</script>

<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
<div class="shrink-0" onclick={hold}>
	<Menu {items} label="Actions for {label}" look="bg-transparent text-ink-3" />
</div>
