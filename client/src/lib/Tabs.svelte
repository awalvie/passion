<script lang="ts">
	import { haptic } from './haptics';

	// Sections of one list, as underlined words. They move between parts of the
	// list; filters live elsewhere. '' is the first tab, which shows every row.
	// The line runs to the edges of the column, whose padding is px-4 on a phone
	// and px-6 beside a pane from lg.
	let {
		value = $bindable(),
		label,
		options
	}: { value: string; label: string; options: { value: string; label: string }[] } = $props();

	let strip: HTMLDivElement;

	// A tab picked by the URL can sit past the right edge, so bring it in once.
	$effect(() => {
		strip.querySelector('[aria-pressed="true"]')?.scrollIntoView({ block: 'nearest', inline: 'nearest' });
	});
</script>

<div
	bind:this={strip}
	role="group"
	aria-label={label}
	class="-mx-4 flex gap-5 overflow-x-auto px-5 shadow-[inset_0_-1px_0_var(--line)] [scrollbar-width:none] lg:-mx-6 lg:px-6"
>
	{#each options as o (o.value)}
		{@const on = value === o.value}
		<button
			type="button"
			aria-pressed={on}
			class="relative flex h-11 shrink-0 items-center text-[15px] font-bold {on ? 'text-ink' : 'text-ink-3'}"
			onclick={() => (value = o.value)}
			use:haptic
		>
			{o.label}
			{#if on}<span class="absolute inset-x-0 bottom-0 h-[3px] rounded-full bg-ink"></span>{/if}
		</button>
	{/each}
</div>
