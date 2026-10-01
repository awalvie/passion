<script lang="ts">
	import { goto } from '$app/navigation';
	import Icon from './Icon.svelte';

	// The installed app on iPhone has no swipe back, so a swipe in from the
	// left edge does what the page's back pill does. An arrow follows the
	// finger; past the mark it fills, and letting go goes back.
	const mark = 80;
	let pull = $state(0);
	let y = $state(0);
	let from: { x: number; y: number; href: string } | null = null;

	const installed =
		matchMedia('(display-mode: standalone)').matches || (navigator as Navigator & { standalone?: boolean }).standalone === true;

	function start(e: TouchEvent) {
		const t = e.touches[0];
		if (!installed || e.touches.length > 1 || t.clientX > 24 || document.querySelector('dialog[open]')) return;
		const back = document.querySelector<HTMLAnchorElement>('a[data-back]');
		if (back) from = { x: t.clientX, y: t.clientY, href: back.href };
	}

	function move(e: TouchEvent) {
		if (!from) return;
		const t = e.touches[0];
		const dx = t.clientX - from.x;
		if (pull === 0 && Math.abs(t.clientY - from.y) > Math.abs(dx)) {
			from = null;
			return;
		}
		pull = Math.max(0, Math.min(mark * 1.4, dx));
		y = t.clientY;
	}

	function end() {
		if (!from) return;
		const href = from.href;
		from = null;
		if (pull >= mark) void goto(href);
		pull = 0;
	}
</script>

<svelte:window
	ontouchstart={start}
	ontouchmove={move}
	ontouchend={end}
	ontouchcancel={() => {
		from = null;
		pull = 0;
	}}
/>

{#if pull > 0}
	<div
		class="pointer-events-none fixed left-0 z-50 -mt-6 flex size-12 items-center justify-center rounded-full shadow-card {pull >= mark ? 'bg-tint text-on-tint' : 'bg-surface text-ink'}"
		style:top="{y}px"
		style:transform="translateX({pull * 0.6 - 48}px)"
		style:opacity={Math.min(1, pull / (mark * 0.6))}
		aria-hidden="true"
	>
		<Icon name="chevron-left" size="1.375rem" stroke={2.4} />
	</div>
{/if}
