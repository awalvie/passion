<script lang="ts">
	import { addDays } from './dates';
	import Icon from './Icon.svelte';
	import type { ScheduledDay } from './plan';

	let { monday, today, week }: { monday: string; today: string; week: ScheduledDay[] } = $props();

	const letters = ['M', 'T', 'W', 'T', 'F', 'S', 'S'];

	const days = $derived(
		letters.map((letter, i) => {
			const date = addDays(monday, i);
			const sessions = week.filter((d) => d.local_date === date);
			return {
				date,
				letter,
				number: Number(date.slice(8)),
				done: sessions.some((d) => d.status === 'done'),
				planned: sessions.length > 0
			};
		})
	);
</script>

<ol class="flex justify-between px-4 pt-3" aria-label="This week">
	{#each days as d (d.date)}
		<li class="flex w-11 flex-col items-center gap-1.5">
			<span class="text-xs font-bold text-ink-2">{d.letter}</span>
			<span
				class="flex size-10 items-center justify-center rounded-full text-[15px] font-bold
					{d.date === today
					? 'bg-tint text-on-tint shadow-[0_0_0_5px_rgba(198,240,91,0.32)]'
					: d.done
						? 'bg-ink text-ground dark:bg-white/15 dark:text-ink'
						: d.planned
							? 'bg-surface shadow-card-sm'
							: 'font-semibold text-ink-2'}"
				aria-label="{d.date}{d.done ? ', done' : d.planned ? ', planned' : ''}"
			>
				{#if d.done && d.date !== today}
					<Icon name="check" size="1.125rem" />
				{:else}
					{d.number}
				{/if}
			</span>
		</li>
	{/each}
</ol>
