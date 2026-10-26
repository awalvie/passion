<script lang="ts" module>
	// pick: a day the person chose. gen: a day a rule made. hold: a day that
	// cannot take it. end: the first or last day of a span.
	export type DayMark = 'pick' | 'gen' | 'hold' | 'end' | '';
</script>

<script lang="ts">
	import { addDays, formatDate, fullDate, gridWeeks, weekdayLetters } from './dates';
	import Icon from './Icon.svelte';

	// A month to pick days on. mark says how a day looks, off hatches a day of
	// time off, and onpick, when given, makes the days from min on tappable.
	let {
		start,
		today,
		color = 'var(--ink)',
		mark,
		off = () => false,
		min,
		onpick
	}: {
		start: string;
		today: string;
		color?: string;
		mark: (date: string) => DayMark;
		off?: (date: string) => boolean;
		min?: string;
		onpick?: (date: string) => void;
	} = $props();

	// The month shown, as its first day. It starts on start's month.
	let shownMonth = $state('');
	const first = $derived(shownMonth || `${start.slice(0, 7)}-01`);
	const lastDay = $derived(addDays(`${addDays(first, 31).slice(0, 7)}-01`, -1));
	const weeks = $derived(gridWeeks(first, lastDay));
	const title = $derived(
		formatDate(first, first.slice(0, 4) === today.slice(0, 4) ? { month: 'long' } : { month: 'long', year: 'numeric' })
	);

	function step(n: number) {
		shownMonth = `${addDays(first, n > 0 ? 31 : -1).slice(0, 7)}-01`;
	}

	const circle: Record<DayMark, string> = {
		pick: 'text-white',
		gen: '',
		hold: 'text-ink-3 line-through shadow-[inset_0_0_0_1.5px_var(--ink-3)]',
		end: 'bg-ink text-ground',
		'': ''
	};
</script>

<div class="flex flex-col">
	<div class="flex h-11 items-center justify-between pl-1">
		<h3 class="text-[15px] font-bold">{title}</h3>
		<span class="flex gap-1">
			<button type="button" class="flex size-11 items-center justify-center rounded-full text-ink-2" aria-label="Earlier month" onclick={() => step(-1)}>
				<Icon name="chevron-left" size="1.25rem" />
			</button>
			<button type="button" class="flex size-11 items-center justify-center rounded-full text-ink-2" aria-label="Later month" onclick={() => step(1)}>
				<Icon name="chevron-right" size="1.25rem" />
			</button>
		</span>
	</div>
	<div class="grid grid-cols-7 text-center">
		{#each weekdayLetters as letter, i (i)}
			<span class="flex h-7 items-center justify-center text-xs font-semibold text-ink-2" aria-hidden="true">{letter}</span>
		{/each}
		{#each weeks.flat() as date (date)}
			{@const m = mark(date)}
			{@const out = date < first || date > lastDay}
			{@const tappable = !!onpick && (!min || date >= min) && m !== 'hold'}
			<button
				type="button"
				class="flex h-11 items-center justify-center font-[family-name:var(--font-digits)] text-[15px] font-semibold
					{off(date) ? 'bg-[repeating-linear-gradient(135deg,var(--hatch)_0_5px,transparent_5px_10px)]' : ''}
					{(out || (min && date < min)) && !m ? 'text-ink-3/45' : 'text-ink'}"
				disabled={!tappable}
				aria-pressed={onpick ? m === 'pick' : undefined}
				aria-label={fullDate(date)}
				onclick={() => onpick?.(date)}
			>
				<i
					class="flex size-[38px] items-center justify-center rounded-full not-italic {circle[m]} {date === today && !m
						? 'shadow-[inset_0_0_0_2px_var(--tint)]'
						: ''}"
					style={m === 'pick' ? `background: ${color}` : m === 'gen' ? `box-shadow: inset 0 0 0 2px ${color}` : undefined}
				>
					{Number(date.slice(8))}
				</i>
			</button>
		{/each}
	</div>
</div>
