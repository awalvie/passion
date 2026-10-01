<script lang="ts" module>
	export type MenuItem = { label: string; danger?: boolean; onclick: () => void };
</script>

<script lang="ts">
	import Icon from './Icon.svelte';

	let {
		items,
		label = 'More',
		look = 'bg-surface text-ink shadow-card-sm',
		size = 'size-11'
	}: { items: MenuItem[]; label?: string; look?: string; size?: string } = $props();

	let open = $state(false);
	let root: HTMLDivElement;

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
		class="flex {size} items-center justify-center rounded-full {look}"
		aria-label={label}
		aria-expanded={open}
		onclick={() => (open = !open)}
	>
		<Icon name="ellipsis" size="1.25rem" />
	</button>
	{#if open}
		<ul class="absolute top-full right-0 z-50 mt-2 min-w-52 overflow-hidden rounded-3xl bg-surface shadow-card">
			{#each items as item (item.label)}
				<li class="[&:not(:first-child)]:shadow-[inset_0_1px_0_var(--line)]">
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
