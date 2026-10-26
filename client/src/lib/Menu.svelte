<script lang="ts" module>
	export type MenuItem = { label: string; danger?: boolean; onclick: () => void };
</script>

<script lang="ts">
	import Icon from './Icon.svelte';

	let {
		items,
		label = 'More',
		look = 'bg-surface text-ink shadow-card-sm'
	}: { items: MenuItem[]; label?: string; look?: string } = $props();

	let open = $state(false);
	let up = $state(false);
	let root: HTMLDivElement;
	let list = $state<HTMLUListElement>();

	// A list that would end under a dock, or off the screen, opens upward.
	$effect(() => {
		if (!list) return;
		const r = list.getBoundingClientRect();
		up = !list.contains(document.elementFromPoint(r.left + r.width / 2, r.bottom - 1));
	});

	function toggle() {
		up = false;
		open = !open;
	}

	function choose(item: MenuItem) {
		open = false;
		item.onclick();
	}

	// A fixed overlay cannot catch the tap: the nav bar's backdrop blur makes
	// the bar, not the screen, the box a fixed child fills.
	function outside(e: PointerEvent) {
		if (open && !root.contains(e.target as Node)) open = false;
	}
</script>

<svelte:window onpointerdown={outside} />

<div class="relative" bind:this={root}>
	<button
		type="button"
		class="flex size-11 items-center justify-center rounded-full {look}"
		aria-label={label}
		aria-expanded={open}
		onclick={toggle}
	>
		<Icon name="ellipsis" size="1.25rem" />
	</button>
	{#if open}
		<ul
			bind:this={list}
			class="absolute right-0 z-50 {up ? 'bottom-full mb-2' : 'top-full mt-2'} min-w-52 overflow-hidden rounded-3xl bg-surface shadow-card"
		>
			{#each items as item (item.label)}
				<li class="list-row">
					<button
						type="button"
						class="w-full px-5 py-3.5 text-left text-[15px] font-bold {item.danger ? 'text-bad' : 'text-ink'}"
						onclick={() => choose(item)}
					>
						{item.label}
					</button>
				</li>
			{/each}
		</ul>
	{/if}
</div>
