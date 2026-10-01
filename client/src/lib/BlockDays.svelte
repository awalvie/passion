<script lang="ts">
	import { addDays, weekday } from './dates';
	import Icon from './Icon.svelte';
	import type { Cycle } from './plan';
	import SessionIcon from './SessionIcon.svelte';
	import type { SessionTemplate } from './template';

	// from is the date day 1 falls on, so each day can name its weekday. A tap
	// on a day opens it below the grid, to add or remove its sessions.
	let {
		days = $bindable(),
		blockDays,
		from,
		templates
	}: { days: Cycle['days']; blockDays: number; from: string; templates: SessionTemplate[] } = $props();

	const byId = $derived(new Map(templates.map((t) => [t.id, t])));
	const list = $derived(Array.from({ length: Math.max(0, Math.min(blockDays, 28)) }, (_, i) => i + 1));
	const on = (day: number) => days.filter((d) => d.day === day);
	const label = (day: number) => `Day ${day}${from ? ` · ${weekday(addDays(from, day - 1))}` : ''}`;

	let chosen = $state(1);
	const open = $derived(Math.min(chosen, list.length));

	function add(day: number, template: string) {
		if (template && !days.some((d) => d.day === day && d.template === template)) {
			days = [...days, { day, template }];
		}
	}
</script>

<div class="grid grid-cols-2 gap-2">
	{#each list as day (day)}
		{@const sessions = on(day)}
		{@const first = sessions[0] && byId.get(sessions[0].template)}
		<button
			type="button"
			class="flex min-h-[60px] items-center gap-2.5 rounded-2xl px-3 py-2 text-left
				{sessions.length ? 'bg-surface shadow-card-sm' : 'shadow-[inset_0_0_0_1px_var(--line)]'}
				{day === open ? 'outline-2 -outline-offset-2 outline-ink' : ''}"
			aria-pressed={day === open}
			onclick={() => (chosen = day)}
		>
			{#if sessions.length}
				<SessionIcon icon={first?.icon ?? null} name={first?.name ?? '?'} size={32} />
			{:else}
				<span class="flex size-8 shrink-0 items-center justify-center rounded-full text-ink-3 outline-[1.5px] -outline-offset-[1.5px] outline-dashed outline-ink-3">
					<Icon name="plus" size="1rem" />
				</span>
			{/if}
			<span class="min-w-0 flex-1">
				<span class="block text-xs font-semibold text-ink-2">{label(day)}</span>
				<span class="flex text-[15px] font-bold {sessions.length ? '' : 'text-ink-2'}">
					<span class="truncate">{sessions.length ? (first?.name ?? 'Retired session') : 'Rest'}</span>
					{#if sessions.length > 1}<span class="shrink-0 pl-1">+{sessions.length - 1}</span>{/if}
				</span>
			</span>
		</button>
	{/each}
</div>

{#if open}
	<section class="rounded-3xl bg-surface px-[18px] pt-3.5 pb-3 shadow-card" aria-label={label(open)}>
		<h3 class="text-[15px] font-bold">{label(open)}</h3>
		{#each on(open) as d (d.template)}
			{@const t = byId.get(d.template)}
			{@const name = t?.name ?? 'Retired session'}
			<div class="flex min-h-[52px] items-center gap-3 py-1.5 [&:not(:first-of-type)]:shadow-[inset_0_1px_0_var(--line)]">
				<SessionIcon icon={t?.icon ?? null} {name} size={32} />
				<span class="min-w-0 flex-1 truncate text-[15px] font-bold">{name}</span>
				<button
					type="button"
					class="flex size-11 shrink-0 items-center justify-center rounded-full text-ink-3"
					aria-label="Remove {name} from day {open}"
					onclick={() => (days = days.filter((x) => x !== d))}
				>
					<Icon name="x" size="1rem" />
				</button>
			</div>
		{/each}
		<select
			class="input mt-2 h-12 px-4"
			aria-label="Add a session on day {open}"
			value=""
			onchange={(e) => {
				add(open, e.currentTarget.value);
				e.currentTarget.value = '';
			}}
		>
			<option value="">Add a session…</option>
			{#each templates as t (t.id)}
				<option value={t.id}>{t.name}</option>
			{/each}
		</select>
	</section>
{/if}
