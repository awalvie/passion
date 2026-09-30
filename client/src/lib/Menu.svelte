<script lang="ts" module>
	export type MenuItem = { label: string; danger?: boolean; onclick: () => void };
</script>

<script lang="ts">
	import Icon from './Icon.svelte';

	let { items, label = 'More' }: { items: MenuItem[]; label?: string } = $props();

	let open = $state(false);

	function choose(item: MenuItem) {
		open = false;
		item.onclick();
	}
</script>

<div class="relative">
	<button
		type="button"
		class="flex size-11 items-center justify-center text-tint"
		aria-label={label}
		aria-expanded={open}
		onclick={() => (open = !open)}
	>
		<Icon name="ellipsis" size="1.5rem" />
	</button>
	{#if open}
		<button
			type="button"
			class="fixed inset-0 z-40 cursor-default"
			aria-label="Close menu"
			onclick={() => (open = false)}
		></button>
		<ul class="absolute top-full right-0 z-50 min-w-48 overflow-hidden rounded-xl bg-surface shadow-lg">
			{#each items as item (item.label)}
				<li class="border-line [&:not(:first-child)]:border-t">
					<button
						type="button"
						class="w-full px-4 py-3 text-left text-base {item.danger ? 'text-bad' : 'text-ink'}"
						onclick={() => choose(item)}
					>
						{item.label}
					</button>
				</li>
			{/each}
		</ul>
	{/if}
</div>
