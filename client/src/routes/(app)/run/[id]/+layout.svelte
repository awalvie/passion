<script lang="ts">
	import type { Snippet } from 'svelte';
	import { listen } from '$lib/audio';

	let { children }: { children: Snippet } = $props();

	// Timer tones may claim the audio only during a run, on any of its pages,
	// so moving between steps keeps it.
	$effect(() => listen());
</script>

{@render children()}
