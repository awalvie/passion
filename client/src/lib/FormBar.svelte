<script lang="ts">
	import type { Snippet } from 'svelte';
	import FormError from './FormError.svelte';

	// As tall as NavBar, so its pills sit where a back pill does. wide is the max
	// width of a page that is wide from lg, so the pills line up with its cards.
	let {
		title,
		cancel,
		cancelLabel = 'Cancel',
		saveLabel = 'Save',
		busyLabel = 'Saving…',
		replace = false,
		heading = 'h1',
		busy = false,
		disabled = false,
		error = '',
		form,
		wide,
		class: cls = '',
		aside,
		end
	}: {
		title: string;
		cancel: string | (() => void);
		cancelLabel?: string;
		saveLabel?: string;
		busyLabel?: string;
		replace?: boolean;
		heading?: 'h1' | 'h2' | 'p';
		busy?: boolean;
		disabled?: boolean;
		error?: string;
		form?: string;
		wide?: number;
		class?: string;
		aside?: Snippet;
		end?: Snippet;
	} = $props();

	const pill = 'flex h-11 items-center justify-self-start rounded-full bg-surface px-4 text-[15px] font-bold text-ink shadow-card-sm';
</script>

<div class="sticky top-0 z-30 bg-ground/85 px-4 pt-[env(safe-area-inset-top)] backdrop-blur-md {wide ? 'lg:px-0' : ''} {cls}">
	<div class="grid h-14 grid-cols-[1fr_auto_1fr] items-center gap-2 {wide ? 'lg:mx-auto lg:px-8' : ''}" style:max-width={wide && `${wide}px`}>
		{#if typeof cancel === 'string'}
			<a class={pill} href={cancel} data-back data-sveltekit-replacestate={replace || undefined} data-sveltekit-noscroll={replace || undefined}>
				{cancelLabel}
			</a>
		{:else}
			<button type="button" class={pill} onclick={cancel}>{cancelLabel}</button>
		{/if}
		<svelte:element this={heading} class="truncate text-[15px] font-bold">{title}</svelte:element>
		<span class="flex items-center gap-4 justify-self-end">
			{@render aside?.()}
			{#if end}
				{@render end()}
			{:else}
				<button
					type="submit"
					{form}
					class="flex h-11 items-center rounded-full bg-tint px-5 text-[15px] font-bold text-on-tint shadow-tint disabled:opacity-50"
					disabled={busy || disabled}
				>
					{busy ? busyLabel : saveLabel}
				</button>
			{/if}
		</span>
	</div>
	{#if error}<div class="pb-2"><FormError message={error} /></div>{/if}
</div>
