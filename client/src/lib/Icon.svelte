<script lang="ts" module>
	type Shape =
		| ['path', { d: string }]
		| ['circle', { cx: string; cy: string; r: string }]
		| ['line', { x1: string; x2: string; y1: string; y2: string }]
		| ['rect', { width: string; height: string; x: string; y: string; rx: string; ry?: string }];

	// The shapes of lucide 0.525.0 (ISC licence), the version V1 loaded as a script.
	const icons = {
		menu: [
			['path', { d: 'M4 12h16' }],
			['path', { d: 'M4 18h16' }],
			['path', { d: 'M4 6h16' }]
		],
		user: [
			['path', { d: 'M19 21v-2a4 4 0 0 0-4-4H9a4 4 0 0 0-4 4v2' }],
			['circle', { cx: '12', cy: '7', r: '4' }]
		]
	} satisfies Record<string, Shape[]>;

	export type IconName = keyof typeof icons;
</script>

<script lang="ts">
	let { name, size = '1rem' }: { name: IconName; size?: string } = $props();
</script>

<svg
	xmlns="http://www.w3.org/2000/svg"
	viewBox="0 0 24 24"
	fill="none"
	stroke="currentColor"
	stroke-width="2"
	stroke-linecap="round"
	stroke-linejoin="round"
	style="width: {size}; height: {size}; flex-shrink: 0"
	aria-hidden="true"
>
	{#each icons[name] as [tag, attrs]}
		{#if tag === 'path'}
			<path {...attrs} />
		{:else if tag === 'circle'}
			<circle {...attrs} />
		{:else if tag === 'line'}
			<line {...attrs} />
		{:else}
			<rect {...attrs} />
		{/if}
	{/each}
</svg>
