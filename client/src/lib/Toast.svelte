<script lang="ts">
	import { fly } from 'svelte/transition';

	// A short note above the tab bar, with one action. It closes by itself
	// after duration milliseconds.
	let {
		message = $bindable(''),
		action,
		onaction,
		duration = 6000
	}: { message?: string; action?: string; onaction?: () => void; duration?: number } = $props();

	const still = matchMedia('(prefers-reduced-motion: reduce)').matches;
	// The note keeps its words while it slides away.
	let text = $state('');
	$effect.pre(() => {
		if (message) text = message;
	});

	// A finger on the note holds it open; a swipe down sends it away.
	let drag = $state<number | null>(null);
	let startY = 0;

	$effect(() => {
		if (!message || drag !== null) return;
		const t = setTimeout(() => (message = ''), duration);
		return () => clearTimeout(t);
	});

	function grab(e: PointerEvent) {
		if ((e.target as Element).closest('button')) return;
		startY = e.clientY;
		drag = 0;
		(e.currentTarget as Element).setPointerCapture(e.pointerId);
	}

	function release() {
		if (drag !== null && drag > 32) message = '';
		drag = null;
	}
</script>

{#if message}
	<div
		class="fixed inset-x-0 bottom-[var(--above-bar)] z-40 mx-auto flex min-h-14 w-[calc(100%-2rem)] max-w-[398px] touch-none items-center gap-3 rounded-full bg-hero py-2 pr-2 pl-5 text-on-hero shadow-card"
		role="status"
		style:translate={drag ? `0 ${drag}px` : undefined}
		onpointerdown={grab}
		onpointermove={(e) => {
			if (drag !== null) drag = Math.max(0, e.clientY - startY);
		}}
		onpointerup={release}
		onpointercancel={() => (drag = null)}
		transition:fly={{ y: 24, duration: still ? 0 : 200 }}
	>
		<span class="min-w-0 flex-1 truncate text-[15px] font-bold">{text}</span>
		{#if action}
			<button
				type="button"
				class="h-11 shrink-0 rounded-full bg-tint px-4 text-[15px] font-bold text-on-tint"
				onclick={() => {
					message = '';
					onaction?.();
				}}
			>
				{action}
			</button>
		{/if}
	</div>
{/if}
