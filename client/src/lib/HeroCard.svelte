<script lang="ts">
	import type { Snippet } from 'svelte';
	import Topo from './Topo.svelte';
	import { heroTopo } from './topo';

	let {
		class: cls = 'mx-4 mt-3.5',
		flush = false,
		children
	}: { class?: string; flush?: boolean; children: Snippet } = $props();
</script>

<article
	class="relative {cls} overflow-hidden rounded-[32px] bg-[radial-gradient(90%_70%_at_88%_0%,var(--hero-2),var(--hero)_70%)] text-on-hero shadow-[0_18px_36px_-12px_rgba(10,30,18,0.55)]"
>
	<Topo shape={heroTopo} class="absolute inset-0 h-full w-full text-[var(--hero-topo)]" />
	<div class="relative flex flex-col {flush ? 'flex-1' : 'p-5'}">
		{@render children()}
	</div>
</article>
