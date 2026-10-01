<script lang="ts">
	let {
		value = $bindable(),
		placeholder,
		label
	}: { value: string; placeholder: string; label: string } = $props();
</script>

<label class="flex h-12 items-center gap-2.5 rounded-full bg-well px-4 text-ink-3 focus-within:shadow-[0_0_0_2px_var(--ink-3)] dark:bg-surface">
	<svg viewBox="0 0 24 24" class="size-[18px] shrink-0" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><circle cx="11" cy="11" r="6.5" /><path d="M16 16l4 4" /></svg>
	<input
		type="search"
		bind:value
		{placeholder}
		aria-label={label}
		autocomplete="off"
		class="h-full min-w-0 flex-1 bg-transparent text-[15px] font-semibold text-ink outline-none placeholder:text-ink-3"
	/>
</label>
