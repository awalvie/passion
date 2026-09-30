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
		primary: 'bg-tint text-on-tint',
		live: 'bg-live text-on-tint',
		secondary: 'bg-surface text-ink shadow-sm',
		danger: 'bg-surface text-bad shadow-sm'
	};

	const cls = $derived(
		`flex h-12 w-full items-center justify-center gap-2 rounded-xl px-5 text-base font-semibold active:opacity-80 disabled:opacity-50 ${looks[variant]}`
	);
</script>

{#if href}
	<a {href} class={cls}>{@render children()}</a>
{:else}
	<button {type} {disabled} {onclick} class={cls}>{@render children()}</button>
{/if}
