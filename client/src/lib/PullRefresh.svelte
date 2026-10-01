<script lang="ts">
	import { invalidateAll } from '$app/navigation';
	import { forget } from './api';
	import Icon from './Icon.svelte';

	// A pull down from the top of a page fetches it again: the installed app
	// has no reload button. Letting go past the mark refreshes.
	const mark = 72;
	let pull = $state(0);
	let busy = $state(false);
	let startY: number | null = null;

	function start(e: TouchEvent) {
		if (busy || scrollY > 0 || e.touches.length > 1 || document.querySelector('dialog[open]')) return;
		if ((e.target as Element).closest('[data-date]')) return;
		startY = e.touches[0].clientY;
	}

	function move(e: TouchEvent) {
		if (startY === null) return;
		const dy = e.touches[0].clientY - startY;
		pull = dy > 0 && scrollY <= 0 ? Math.min(mark * 1.5, dy * 0.5) : 0;
	}

	async function end() {
		if (startY === null) return;
		startY = null;
		if (pull < mark) {
			pull = 0;
			return;
		}
		busy = true;
		forget();
		try {
			await invalidateAll();
		} finally {
			busy = false;
			pull = 0;
		}
	}
</script>

<svelte:window
	ontouchstart={start}
	ontouchmove={move}
	ontouchend={end}
	ontouchcancel={() => {
		startY = null;
		if (!busy) pull = 0;
	}}
/>

{#if pull > 0 || busy}
	<div
		class="pointer-events-none fixed inset-x-0 top-[calc(env(safe-area-inset-top)+0.5rem)] z-40 flex justify-center"
		style:transform="translateY({busy ? mark * 0.5 : pull * 0.5}px)"
		style:opacity={busy ? 1 : Math.min(1, pull / mark)}
		role={busy ? 'progressbar' : undefined}
		aria-label={busy ? 'Refreshing' : undefined}
	>
		<span
			class="flex size-10 items-center justify-center rounded-full bg-surface text-ink shadow-card {busy ? 'motion-safe:animate-spin' : ''} {pull >= mark ? 'bg-tint text-on-tint' : ''}"
			style:rotate={busy ? undefined : `${pull * 4}deg`}
		>
			<Icon name="repeat" size="1.125rem" />
		</span>
	</div>
{/if}
