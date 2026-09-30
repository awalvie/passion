<script lang="ts">
	import TabBar from '$lib/TabBar.svelte';

	let { children } = $props();
</script>

<div class="pb-[calc(3.5rem+env(safe-area-inset-bottom)+1rem)]">
	{@render children()}
</div>

<TabBar />
