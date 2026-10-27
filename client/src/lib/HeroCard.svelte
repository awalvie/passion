<script lang="ts">
	import type { Snippet } from 'svelte';
	import Topo from './Topo.svelte';
	import { heroTopo } from './topo';

	let { class: cls = 'mx-4 mt-3.5', children }: { class?: string; children: Snippet } = $props();
</script>

<!-- Only the lines are clipped, so a menu in the card can open past its edge. -->
<article
	class="relative {cls} rounded-[32px] bg-[radial-gradient(90%_70%_at_88%_0%,var(--hero-2),var(--hero)_70%)] text-on-hero shadow-[0_18px_36px_-12px_rgba(10,30,18,0.55)]"
>
	<div class="absolute inset-0 overflow-hidden rounded-[inherit]">
		<Topo shape={heroTopo} class="h-full w-full text-[var(--hero-topo)]" />
	</div>
	<div class="relative flex flex-col p-5">
		{@render children()}
	</div>
</article>
