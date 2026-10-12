<script lang="ts">
	import Topo from '$lib/Topo.svelte';
	import { topTopo } from '$lib/topo';

	let { children } = $props();
</script>

<Topo
	shape={topTopo}
	class="absolute inset-x-0 top-0 h-80 w-full text-[var(--topo)] [mask-image:linear-gradient(#000_30%,transparent)]"
/>
<section class="relative mx-auto flex max-w-[430px] flex-col px-4 pt-[calc(env(safe-area-inset-top)+4rem)] pb-8">
	{@render children()}
</section>
