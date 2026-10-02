<script lang="ts" module>
	type Shape =
		| ['path', { d: string; fill?: string; stroke?: string }]
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
		'chevron-left': [['path', { d: 'm15 18-6-6 6-6' }]],
		'chevron-right': [['path', { d: 'm9 18 6-6-6-6' }]],
		// Filled, unlike lucide's, as play buttons are drawn.
		play: [['path', { d: 'M8 5.5v13l10.5-6.5z', fill: 'currentColor', stroke: 'none' }]],
		repeat: [
			['path', { d: 'm17 2 4 4-4 4' }],
			['path', { d: 'M3 11v-1a4 4 0 0 1 4-4h14' }],
			['path', { d: 'm7 22-4-4 4-4' }],
			['path', { d: 'M21 13v1a4 4 0 0 1-4 4H3' }]
		],
		check: [['path', { d: 'M20 6 9 17l-5-5' }]],
		'map-pin': [
			['path', { d: 'M20 10c0 4.993-5.539 10.193-7.399 11.799a1 1 0 0 1-1.202 0C9.539 20.193 4 14.993 4 10a8 8 0 0 1 16 0' }],
			['circle', { cx: '12', cy: '10', r: '3' }]
		],
		'chevron-down': [['path', { d: 'm6 9 6 6 6-6' }]],
		lock: [
			['rect', { width: '18', height: '11', x: '3', y: '11', rx: '2', ry: '2' }],
			['path', { d: 'M7 11V7a5 5 0 0 1 10 0v4' }]
		],
		'undo-2': [
			['path', { d: 'M9 14 4 9l5-5' }],
			['path', { d: 'M4 9h10.5a5.5 5.5 0 0 1 0 11H11' }]
		],
		x: [
			['path', { d: 'M18 6 6 18' }],
			['path', { d: 'm6 6 12 12' }]
		],
		ellipsis: [
			['circle', { cx: '12', cy: '12', r: '1' }],
			['circle', { cx: '19', cy: '12', r: '1' }],
			['circle', { cx: '5', cy: '12', r: '1' }]
		],
		hand: [
			['path', { d: 'M18 11V6a2 2 0 0 0-2-2a2 2 0 0 0-2 2' }],
			['path', { d: 'M14 10V4a2 2 0 0 0-2-2a2 2 0 0 0-2 2v2' }],
			['path', { d: 'M10 10.5V6a2 2 0 0 0-2-2a2 2 0 0 0-2 2v8' }],
			['path', { d: 'M18 8a2 2 0 1 1 4 0v6a8 8 0 0 1-8 8h-2c-2.8 0-4.5-.86-5.99-2.34l-3.6-3.6a2 2 0 0 1 2.83-2.82L7 15' }]
		],
		dumbbell: [
			[
				'path',
				{
					d: 'M17.596 12.768a2 2 0 1 0 2.829-2.829l-1.768-1.767a2 2 0 0 0 2.828-2.829l-2.828-2.828a2 2 0 0 0-2.829 2.828l-1.767-1.768a2 2 0 1 0-2.829 2.829z'
				}
			],
			['path', { d: 'm2.5 21.5 1.4-1.4' }],
			['path', { d: 'm20.1 3.9 1.4-1.4' }],
			[
				'path',
				{
					d: 'M5.343 21.485a2 2 0 1 0 2.829-2.828l1.767 1.768a2 2 0 1 0 2.829-2.829l-6.364-6.364a2 2 0 1 0-2.829 2.829l1.768 1.767a2 2 0 0 0-2.828 2.829z'
				}
			],
			['path', { d: 'm9.6 14.4 4.8-4.8' }]
		],
		'grid-3x3': [
			['rect', { width: '18', height: '18', x: '3', y: '3', rx: '2' }],
			['path', { d: 'M3 9h18' }],
			['path', { d: 'M3 15h18' }],
			['path', { d: 'M9 3v18' }],
			['path', { d: 'M15 3v18' }]
		],
		'person-standing': [
			['circle', { cx: '12', cy: '5', r: '1' }],
			['path', { d: 'm9 20 3-6 3 6' }],
			['path', { d: 'm6 8 6 2 6-2' }],
			['path', { d: 'M12 10v4' }]
		],
		flame: [
			[
				'path',
				{
					d: 'M8.5 14.5A2.5 2.5 0 0 0 11 12c0-1.38-.5-2-1-3-1.072-2.143-.224-4.054 2-6 .5 2.5 2 4.9 4 6.5 2 1.6 3 3.5 3 5.5a7 7 0 1 1-14 0c0-1.153.433-2.294 1-3a2.5 2.5 0 0 0 2.5 2.5z'
				}
			]
		],
		'heart-pulse': [
			[
				'path',
				{ d: 'M19 14c1.49-1.46 3-3.21 3-5.5A5.5 5.5 0 0 0 16.5 3c-1.76 0-3 .5-4.5 2-1.5-1.5-2.74-2-4.5-2A5.5 5.5 0 0 0 2 8.5c0 2.3 1.5 4.05 3 5.5l7 7Z' }
			],
			['path', { d: 'M3.22 12H9.5l.5-1 2 4.5 2-7 1.5 3.5h5.27' }]
		],
		zap: [
			[
				'path',
				{
					d: 'M4 14a1 1 0 0 1-.78-1.63l9.9-10.2a.5.5 0 0 1 .86.46l-1.92 6.02A1 1 0 0 0 13 10h7a1 1 0 0 1 .78 1.63l-9.9 10.2a.5.5 0 0 1-.86-.46l1.92-6.02A1 1 0 0 0 11 14z'
				}
			]
		],
		moon: [['path', { d: 'M12 3a6 6 0 0 0 9 9 9 9 0 1 1-9-9Z' }]],
		target: [
			['circle', { cx: '12', cy: '12', r: '10' }],
			['circle', { cx: '12', cy: '12', r: '6' }],
			['circle', { cx: '12', cy: '12', r: '2' }]
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
	let { name, size = '1rem', stroke = 2 }: { name: IconName; size?: string; stroke?: number } = $props();
</script>

<svg
	xmlns="http://www.w3.org/2000/svg"
	viewBox="0 0 24 24"
	fill="none"
	stroke="currentColor"
	stroke-width={stroke}
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
