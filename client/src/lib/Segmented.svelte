<script lang="ts">
	import { haptic } from './haptics';

	// One switch between views: links when each view has an address, buttons
	// otherwise. replace keeps the switch out of the back history.
	type Item = { label: string; on: boolean; href?: string; onclick?: () => void };
	let { items, label, replace = false }: { items: Item[]; label: string; replace?: boolean } = $props();

	const cls = (on: boolean) =>
		`flex h-11 items-center justify-center rounded-full text-[15px] font-bold ${on ? 'bg-surface text-ink shadow-card-sm' : 'text-ink-2'}`;
</script>

<svelte:element
	this={items[0]?.href ? 'nav' : 'div'}
	role={items[0]?.href ? undefined : 'group'}
	class="grid gap-1 rounded-full bg-well p-1"
	style:grid-template-columns="repeat({items.length}, 1fr)"
	aria-label={label}
>
	{#each items as item (item.label)}
		{#if item.href}
			<a
				href={item.href}
				class={cls(item.on)}
				aria-current={item.on ? 'page' : undefined}
				data-sveltekit-replacestate={replace || undefined}
				data-sveltekit-noscroll={replace || undefined}
			>
				{item.label}
			</a>
		{:else}
			<button type="button" class={cls(item.on)} aria-pressed={item.on} onclick={item.onclick} use:haptic>{item.label}</button>
		{/if}
	{/each}
</svelte:element>
