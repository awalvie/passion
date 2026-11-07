<script lang="ts">
	import type { Snippet } from 'svelte';
	import { page } from '$app/state';
	import { listen, release } from '$lib/audio';

	let { children }: { children: Snippet } = $props();
	// A derived id changes only with the run, not with each step's page.
	const id = $derived(page.params.id!);
	const finished = $derived(page.route.id?.endsWith('/finish') ?? false);

	// Timer tones may claim the audio only during a run, on any of its pages,
	// so moving between steps keeps it. The finish page has no timer, and
	// History opens it long after, so it hands the audio back.
	$effect(() => (finished ? release() : listen(id)));
</script>

{@render children()}
