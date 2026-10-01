<script lang="ts">
	import type { Snippet } from 'svelte';

	type Variant = 'primary' | 'live' | 'secondary' | 'danger';

	let {
		variant = 'primary',
		href,
		type = 'button',
		disabled = false,
		onclick,
		children
	}: {
		variant?: Variant;
		href?: string;
		type?: 'button' | 'submit';
		disabled?: boolean;
		onclick?: (e: MouseEvent) => void;
		children: Snippet;
	} = $props();

	const looks: Record<Variant, string> = {
		primary: 'h-14 bg-tint text-xl text-on-tint shadow-tint',
		live: 'h-14 bg-live text-xl text-on-live shadow-live',
		secondary: 'h-12 bg-surface text-[15px] text-ink shadow-card',
		danger: 'h-12 bg-surface text-[15px] text-bad shadow-card'
	};

	const cls = $derived(
		`flex w-full items-center justify-center gap-2 whitespace-nowrap rounded-full px-5 font-bold tracking-tight active:opacity-80 disabled:opacity-50 ${looks[variant]}`
	);
</script>

{#if href}
	<a {href} class={cls}>{@render children()}</a>
{:else}
	<button {type} {disabled} {onclick} class={cls}>{@render children()}</button>
{/if}
