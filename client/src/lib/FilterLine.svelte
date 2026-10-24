<script lang="ts">
	import { plural } from './text';

	// The line above a filtered list: how much is left of the part you are in,
	// and a way out. Clear leaves the tab alone.
	let { shown, total, filters, onclear }: { shown: number; total: number; filters: number; onclear: () => void } = $props();
</script>

<div class="flex items-center justify-between gap-3 px-1">
	<span class="min-w-0 truncate text-xs font-bold text-ink-2">
		{shown} of {total}{filters ? ` · ${plural(filters, 'filter')}` : ''}
	</span>
	<button
		type="button"
		class="relative shrink-0 text-xs font-extrabold text-ink underline underline-offset-[3px] before:absolute before:-inset-x-2 before:-inset-y-4 before:content-['']"
		onclick={onclear}
	>
		Clear
	</button>
</div>
