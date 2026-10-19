<script lang="ts">
	import type { Origin } from './exercise';
	import Icon from './Icon.svelte';

	// Built-in is the plain case, so it keeps the space and draws nothing.
	let { origin }: { origin: Origin } = $props();
</script>

{#if origin === 'builtin'}
	<span class="size-6 shrink-0" aria-hidden="true"></span>
{:else}
	<span class="flex size-6 shrink-0 items-center justify-center rounded-full bg-well text-ink-2">
		<Icon name={origin === 'private' ? 'lock' : 'user'} size="12px" stroke={2.4} />
		<span class="sr-only">{origin === 'private' ? '. Private' : '. Yours'}</span>
	</span>
{/if}
