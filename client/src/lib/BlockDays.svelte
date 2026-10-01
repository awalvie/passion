<script lang="ts">
	import { addDays, weekday } from './dates';
	import type { Cycle } from './plan';
	import type { SessionTemplate } from './template';

	// from is the date day 1 falls on, so each day can name its weekday.
	let {
		days = $bindable(),
		blockDays,
		from,
		templates
	}: { days: Cycle['days']; blockDays: number; from: string; templates: SessionTemplate[] } = $props();

	const names = $derived(new Map(templates.map((t) => [t.id, t.name])));
	const list = $derived(Array.from({ length: Math.max(0, Math.min(blockDays, 28)) }, (_, i) => i + 1));

	function add(day: number, template: string) {
		if (template && !days.some((d) => d.day === day && d.template === template)) {
			days = [...days, { day, template }];
		}
	}
</script>

<ul>
	{#each list as day (day)}
		<li class="flex flex-col gap-2 py-3 [&:not(:first-child)]:shadow-[inset_0_1px_0_var(--line)]">
			<span class="text-xs font-semibold text-ink-2">Day {day}{from ? ` · ${weekday(addDays(from, day - 1))}` : ''}</span>
			{#each days.filter((d) => d.day === day) as d (d.template)}
				{@const name = names.get(d.template) ?? 'Retired session'}
				<span class="flex items-center gap-2">
					<span class="min-w-0 flex-1 truncate text-[15px] font-bold">{name}</span>
					<button
						type="button"
						class="h-11 shrink-0 px-2 text-[15px] font-bold text-bad"
						aria-label="Remove {name} from day {day}"
						onclick={() => (days = days.filter((x) => x !== d))}
					>
						Remove
					</button>
				</span>
			{/each}
			<select
				class="input h-12 px-4"
				aria-label="Add a session on day {day}"
				value=""
				onchange={(e) => {
					add(day, e.currentTarget.value);
					e.currentTarget.value = '';
				}}
			>
				<option value="">Add a session…</option>
				{#each templates as t (t.id)}
					<option value={t.id}>{t.name}</option>
				{/each}
			</select>
		</li>
	{/each}
</ul>
