<script lang="ts">
	import { untrack } from 'svelte';

	let { digit, wide = false, look }: { digit: string; wide?: boolean; look: { card: string; digit: string; split: string } } =
		$props();

	let last = untrack(() => digit);
	let from = $state<string | null>(null);

	$effect.pre(() => {
		if (digit === last) return;
		const old = last;
		last = digit;
		if (matchMedia('(prefers-reduced-motion: reduce)').matches) return;
		from = old;
		const done = setTimeout(() => (from = null), 250);
		return () => clearTimeout(done);
	});
</script>

<div class="relative h-(--h) shrink-0 {wide ? 'w-[calc(var(--h)*0.747)]' : 'w-[calc(var(--h)*0.449)]'}">
	<div
		class="relative size-full overflow-hidden rounded-[28px] text-center font-[family-name:var(--font-digits)] {wide ? 'text-[length:calc(var(--h)*1.1)]' : 'text-[length:calc(var(--h)*0.754)]'} leading-(--h) font-bold tabular-nums shadow-[0_26px_40px_-18px_rgba(0,0,0,0.55)] {look.card} {look.digit}"
	>
		<div class="absolute inset-x-0 top-0 h-1/2 bg-linear-to-b from-white/8 to-transparent"></div>
		<span class="relative">{digit}</span>
		{#if from !== null}
			{#key digit}
				<div class="absolute inset-x-0 bottom-0 h-1/2 overflow-hidden {look.card}">
					<span class="absolute inset-x-0 bottom-0 h-(--h)">{from}</span>
				</div>
				<div class="leaf top-leaf absolute inset-x-0 top-0 h-1/2 overflow-hidden {look.card}">
					<i class="absolute inset-0 bg-linear-to-b from-white/8 to-transparent"></i>
					<span class="absolute inset-x-0 top-0 h-(--h)">{from}</span>
				</div>
				<div class="leaf bottom-leaf absolute inset-x-0 bottom-0 h-1/2 overflow-hidden {look.card}">
					<span class="absolute inset-x-0 bottom-0 h-(--h)">{digit}</span>
					<i class="absolute inset-0 bg-linear-to-b from-black/40 to-black/5"></i>
				</div>
			{/key}
		{/if}
		<div class="absolute inset-x-0 top-1/2 h-0.5 -translate-y-1/2 {look.split}"></div>
	</div>
	<i class="absolute top-1/2 -left-0.75 h-3.5 w-1.25 -translate-y-1/2 rounded-sm {look.card}"></i>
	<i class="absolute top-1/2 -right-0.75 h-3.5 w-1.25 -translate-y-1/2 rounded-sm {look.card}"></i>
</div>

<style>
	.leaf {
		--fold: perspective(calc(var(--h) * 2.4));
	}
	.top-leaf {
		transform-origin: 50% 100%;
		transform: var(--fold) rotateX(0deg);
		animation: fold-top 125ms ease-in forwards;
	}
	.bottom-leaf {
		transform-origin: 50% 0;
		transform: var(--fold) rotateX(90deg);
		animation: fold-bottom 125ms ease-out 125ms forwards;
	}
	@keyframes fold-top {
		to {
			transform: var(--fold) rotateX(-90deg);
		}
	}
	@keyframes fold-bottom {
		to {
			transform: var(--fold) rotateX(0deg);
		}
	}
</style>
