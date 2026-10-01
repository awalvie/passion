<script lang="ts" module>
	import type { IconName } from './Icon.svelte';

	// The icons a session template can carry, in the order the editor offers them.
	export const sessionIcons = [
		['hand', 'Fingers'],
		['dumbbell', 'Strength'],
		['mountain', 'Climbing'],
		['grid-3x3', 'Board'],
		['zap', 'Power'],
		['heart-pulse', 'Endurance'],
		['flame', 'Core'],
		['person-standing', 'Mobility'],
		['target', 'Limit'],
		['moon', 'Recovery']
	] as const satisfies readonly (readonly [IconName, string])[];

	const known = new Set<string>(sessionIcons.map(([name]) => name));

	export type IconState = 'done' | 'started' | 'missed' | 'planned' | 'today';
</script>

<script lang="ts">
	import Icon from './Icon.svelte';

	let {
		icon,
		name,
		state = 'planned',
		size = 32
	}: { icon: string | null; name: string; state?: IconState; size?: number } = $props();

	const looks: Record<IconState, string> = {
		done: 'bg-ink text-ground dark:bg-white/15 dark:text-ink',
		started: 'bg-ink text-ground dark:bg-white/15 dark:text-ink',
		missed: 'text-ink-3 outline-[1.5px] -outline-offset-[1.5px] outline-dashed outline-ink-3',
		planned: 'bg-surface text-ink shadow-card-sm',
		today: 'bg-tint text-on-tint'
	};
</script>

<span
	class="flex shrink-0 items-center justify-center rounded-full font-bold {looks[state]}"
	style="width: {size}px; height: {size}px; font-size: {Math.round(size * 0.44)}px"
	aria-hidden="true"
>
	{#if icon && known.has(icon)}
		<Icon name={icon as IconName} size="{Math.round(size * 0.5)}px" />
	{:else}
		{name.trim().charAt(0).toUpperCase()}
	{/if}
</span>
