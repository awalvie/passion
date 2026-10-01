<script lang="ts" module>
	type Shape =
		| ['path', { d: string }]
		| ['circle', { cx: string; cy: string; r: string }]
		| ['line', { x1: string; x2: string; y1: string; y2: string }]
		| ['rect', { width: string; height: string; x: string; y: string; rx: string; ry?: string }];

	// The shapes of lucide 0.525.0 (ISC licence), the version V1 loaded as a script.
	const icons = {
		'list-checks': [
			['path', { d: 'm3 17 2 2 4-4' }],
			['path', { d: 'm3 7 2 2 4-4' }],
			['path', { d: 'M13 6h8' }],
			['path', { d: 'M13 12h8' }],
			['path', { d: 'M13 18h8' }]
		],
		timer: [
			['line', { x1: '10', x2: '14', y1: '2', y2: '2' }],
			['line', { x1: '12', x2: '15', y1: '14', y2: '11' }],
			['circle', { cx: '12', cy: '14', r: '8' }]
		],
		mountain: [['path', { d: 'm8 3 4 8 5-5 5 15H2L8 3z' }]],
		backpack: [
			['path', { d: 'M4 10a4 4 0 0 1 4-4h8a4 4 0 0 1 4 4v10a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2z' }],
			['path', { d: 'M8 10h8' }],
			['path', { d: 'M8 18h8' }],
			['path', { d: 'M8 22v-6a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v6' }],
			['path', { d: 'M9 6V4a2 2 0 0 1 2-2h2a2 2 0 0 1 2 2v2' }]
		],
		layers: [
			[
				'path',
				{
					d: 'M12.83 2.18a2 2 0 0 0-1.66 0L2.6 6.08a1 1 0 0 0 0 1.83l8.58 3.91a2 2 0 0 0 1.66 0l8.58-3.9a1 1 0 0 0 0-1.83z'
				}
			],
			['path', { d: 'M2 12a1 1 0 0 0 .58.91l8.6 3.91a2 2 0 0 0 1.65 0l8.58-3.9A1 1 0 0 0 22 12' }],
			['path', { d: 'M2 17a1 1 0 0 0 .58.91l8.6 3.91a2 2 0 0 0 1.65 0l8.58-3.9A1 1 0 0 0 22 17' }]
		],
		'arrow-left': [
			['path', { d: 'm12 19-7-7 7-7' }],
			['path', { d: 'M19 12H5' }]
		],
		image: [
			['rect', { width: '18', height: '18', x: '3', y: '3', rx: '2', ry: '2' }],
			['circle', { cx: '9', cy: '9', r: '2' }],
			['path', { d: 'm21 15-3.086-3.086a2 2 0 0 0-2.828 0L6 21' }]
		],
		'arrow-up': [
			['path', { d: 'm5 12 7-7 7 7' }],
			['path', { d: 'M12 19V5' }]
		],
		'arrow-down': [
			['path', { d: 'M12 5v14' }],
			['path', { d: 'm19 12-7 7-7-7' }]
		],
		'trash-2': [
			['path', { d: 'M3 6h18' }],
			['path', { d: 'M19 6v14c0 1-1 2-2 2H7c-1 0-2-1-2-2V6' }],
			['path', { d: 'M8 6V4c0-1 1-2 2-2h4c1 0 2 1 2 2v2' }],
			['line', { x1: '10', x2: '10', y1: '11', y2: '17' }],
			['line', { x1: '14', x2: '14', y1: '11', y2: '17' }]
		],
		pencil: [
			[
				'path',
				{
					d: 'M21.174 6.812a1 1 0 0 0-3.986-3.987L3.842 16.174a2 2 0 0 0-.5.83l-1.321 4.352a.5.5 0 0 0 .623.622l4.353-1.32a2 2 0 0 0 .83-.497z'
				}
			],
			['path', { d: 'm15 5 4 4' }]
		],
		'book-marked': [
			['path', { d: 'M10 2v8l3-3 3 3V2' }],
			[
				'path',
				{ d: 'M4 19.5v-15A2.5 2.5 0 0 1 6.5 2H19a1 1 0 0 1 1 1v18a1 1 0 0 1-1 1H6.5a1 1 0 0 1 0-5H20' }
			]
		],
		peak: [
			['path', { d: 'M3 19l6-9 4 5.5 2.5-3L21 19z' }],
			['circle', { cx: '16.5', cy: '6.5', r: '2' }]
		],
		stack: [
			['rect', { width: '16', height: '6', x: '4', y: '4', rx: '2' }],
			['rect', { width: '16', height: '6', x: '4', y: '14', rx: '2' }]
		],
		clock: [
			['circle', { cx: '12', cy: '12', r: '8.5' }],
			['path', { d: 'M12 7.5V12l3 2' }]
		],
		plus: [['path', { d: 'M12 5v14M5 12h14' }]],
		list: [
			['path', { d: 'M9 7h11M9 12h11M9 17h11' }],
			['circle', { cx: '4.5', cy: '7', r: '1' }],
			['circle', { cx: '4.5', cy: '12', r: '1' }],
			['circle', { cx: '4.5', cy: '17', r: '1' }]
		],
		layers2: [
			['path', { d: 'M12 4l8.5 4.5L12 13 3.5 8.5z' }],
			['path', { d: 'M3.5 12.5L12 17l8.5-4.5' }],
			['path', { d: 'M3.5 16.5L12 21l8.5-4.5' }]
		],
		calendar: [
			['path', { d: 'M8 2v4' }],
			['path', { d: 'M16 2v4' }],
			['rect', { width: '18', height: '18', x: '3', y: '4', rx: '2' }],
			['path', { d: 'M3 10h18' }]
		],
		'chevron-right': [['path', { d: 'm9 18 6-6-6-6' }]],
		check: [['path', { d: 'M20 6 9 17l-5-5' }]],
		x: [
			['path', { d: 'M18 6 6 18' }],
			['path', { d: 'm6 6 12 12' }]
		],
		ellipsis: [
			['circle', { cx: '12', cy: '12', r: '1' }],
			['circle', { cx: '19', cy: '12', r: '1' }],
			['circle', { cx: '5', cy: '12', r: '1' }]
		],
		settings: [
			[
				'path',
				{
					d: 'M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.1a2 2 0 0 1 1 1.72v.51a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 0 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.39a2 2 0 0 0-.73-2.73l-.15-.08a2 2 0 0 1-1-1.74v-.5a2 2 0 0 1 1-1.74l.15-.09a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2z'
				}
			],
			['circle', { cx: '12', cy: '12', r: '3' }]
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
