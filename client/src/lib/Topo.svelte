<script lang="ts">
	let { shape, class: cls = '' }: { shape: { viewBox: string; d: string }; class?: string } = $props();
</script>

<svg viewBox={shape.viewBox} preserveAspectRatio="xMidYMin slice" class="pointer-events-none {cls}" aria-hidden="true">
	<path d={shape.d} fill="none" stroke="currentColor" stroke-width="1.1" />
</svg>
