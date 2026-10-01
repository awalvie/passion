<script lang="ts">
	let { digit, wide = false, look }: { digit: string; wide?: boolean; look: { card: string; digit: string; split: string } } =
		$props();
</script>

<div class="relative h-(--h) shrink-0 {wide ? 'w-[calc(var(--h)*0.672)]' : 'w-[calc(var(--h)*0.448)]'}">
	<div
		class="relative size-full overflow-hidden rounded-3xl text-center {wide ? 'text-[length:calc(var(--h)*0.84)] tracking-tight' : 'text-[length:calc(var(--h)*0.7)] tracking-tighter'} leading-(--h) font-bold tabular-nums shadow-[0_26px_40px_-18px_rgba(0,0,0,0.55)] {look.card} {look.digit}"
	>
		<div class="absolute inset-x-0 top-0 h-1/2 bg-linear-to-b from-white/8 to-transparent"></div>
		<span class="relative">{digit}</span>
		<div class="absolute inset-x-0 top-1/2 h-0.5 -translate-y-1/2 {look.split}"></div>
	</div>
	<i class="absolute top-1/2 -left-0.75 h-3.5 w-1.25 -translate-y-1/2 rounded-sm {look.card}"></i>
	<i class="absolute top-1/2 -right-0.75 h-3.5 w-1.25 -translate-y-1/2 rounded-sm {look.card}"></i>
</div>
