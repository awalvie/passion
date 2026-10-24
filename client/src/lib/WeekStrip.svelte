<script lang="ts">
	import { addDays, weekdayLetters } from './dates';
	import Icon from './Icon.svelte';
	import type { ScheduledDay } from './plan';

	let {
		monday,
		today,
		week,
		// Given on Today, where a tap on a day opens it in place of today.
		selected,
		// Dates with a finished run, so days trained without a plan show done too.
		ran
	}: { monday: string; today: string; week: ScheduledDay[]; selected?: string; ran?: string[] } = $props();


	const days = $derived(
		weekdayLetters.map((letter, i) => {
			const date = addDays(monday, i);
			const sessions = week.filter((d) => d.local_date === date);
			return {
				date,
				letter,
				number: Number(date.slice(8)),
				done: sessions.some((d) => d.status === 'done') || !!ran?.includes(date),
				missed: sessions.some((d) => d.status === 'missed'),
				planned: sessions.length > 0
			};
		})
	);
</script>

<ol class="flex justify-between px-4 pt-3" aria-label="This week">
	{#each days as d (d.date)}
		<li class="w-11">
			<svelte:element
				this={selected && (d.date === today || d.planned || d.done) ? 'a' : 'div'}
				href={!selected ? undefined : d.date === today ? '/' : d.planned || d.done ? `/?day=${d.date}` : undefined}
				class="flex flex-col items-center gap-1.5"
				data-sveltekit-replacestate
				data-sveltekit-noscroll
				aria-label="{d.date}{d.done ? ', done' : d.missed ? ', missed' : d.planned ? ', planned' : ''}"
				aria-current={d.date === selected ? 'date' : undefined}
			>
				<span class="text-xs font-bold text-ink-2">{d.letter}</span>
				<span
					class="flex size-10 items-center justify-center rounded-full text-[15px] font-bold
						{d.date === today
						? 'bg-tint text-on-tint shadow-[0_0_0_5px_rgba(198,240,91,0.32)]'
						: d.done
							? 'bg-ink text-ground dark:bg-white/15 dark:text-ink'
							: d.missed
								? 'border-2 border-dashed border-ink-3 text-ink-2'
								: d.planned
									? 'bg-surface shadow-card-sm'
									: 'font-semibold text-ink-2'}
						{d.date === selected && d.date !== today ? 'ring-[2.5px] ring-ink' : ''}"
				>
					{#if d.done && d.date !== today}
						<Icon name="check" size="1.125rem" />
					{:else}
						{d.number}
					{/if}
				</span>
			</svelte:element>
		</li>
	{/each}
</ol>
