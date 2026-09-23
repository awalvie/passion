<script lang="ts">
	import type { Snippet } from 'svelte';
	import { choiceMeta, stepMeta, type Choice } from '$lib/template';

	let { choice, actions }: { choice: Choice; actions: Snippet } = $props();
</script>

<details class="card-muted overflow-hidden passion-disclosure">
	<summary class="cursor-pointer list-none flex items-center justify-between gap-3 px-3 py-2.5">
		<div class="min-w-0 flex-1">
			<div class="text-sm font-medium truncate">{choice.name}</div>
			<div class="text-xs muted truncate">{choiceMeta(choice)}</div>
		</div>
		{@render actions()}
	</summary>

	<ul class="px-3 pb-3 space-y-1">
		{#each choice.options as option, k (k)}
			<li>
				<div class="text-xs font-medium break-words">{option.name}</div>
				<div class="text-[11px] muted">{stepMeta(option)}</div>
			</li>
		{/each}
	</ul>
</details>
