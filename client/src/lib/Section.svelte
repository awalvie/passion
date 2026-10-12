<script lang="ts">
	import type { Snippet } from 'svelte';

	// aside sits at the end of the heading's line, such as a count or a link.
	let { title, aside, children }: { title: string; aside?: Snippet; children: Snippet } = $props();
</script>

<section class="flex flex-col gap-2">
	<div class="flex items-baseline justify-between px-1">
		<h2 class="text-[15px] font-bold">{title}</h2>
		{@render aside?.()}
	</div>
	{@render children()}
</section>
