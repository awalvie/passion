<script lang="ts">
	import Button from './Button.svelte';
	import { addDays, weekday } from './dates';
	import Icon from './Icon.svelte';
	import type { Cycle } from './plan';
	import SessionIcon from './SessionIcon.svelte';
	import Sheet from './Sheet.svelte';
	import type { SessionTemplate } from './template';

	// from is the date day 1 falls on, so each day can name its weekday. A tap
	// on a day opens a sheet to add or remove its sessions.
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

	let open = $state(false);
	let chosen = $state(1);

	// A retired session stays listed on its day until it is taken off.
	const retired = $derived(on(chosen).filter((d) => !byId.has(d.template)));

	function toggle(template: string) {
		const has = days.some((d) => d.day === chosen && d.template === template);
		days = has ? days.filter((d) => !(d.day === chosen && d.template === template)) : [...days, { day: chosen, template }];
	}
</script>

<div class="grid grid-cols-2 gap-2">
	{#each list as day (day)}
		{@const sessions = on(day)}
		{@const first = sessions[0] && byId.get(sessions[0].template)}
		<button
			type="button"
			class="flex min-h-[60px] items-center gap-2.5 rounded-2xl px-3 py-2 text-left
				{sessions.length ? 'bg-surface shadow-card-sm' : 'shadow-[inset_0_0_0_1px_var(--line)]'}"
			onclick={() => {
				chosen = day;
				open = true;
			}}
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
<p class="px-1 text-center text-xs font-semibold text-ink-2">Tap a day to add or remove sessions.</p>

<Sheet bind:open title={label(chosen)}>
	<ul class="rounded-3xl bg-surface px-[18px] shadow-card">
		{#each [...templates.map((t) => ({ id: t.id, name: t.name, icon: t.icon })), ...retired.map((d) => ({ id: d.template, name: 'Retired session', icon: null }))] as t (t.id)}
			{@const picked = days.some((d) => d.day === chosen && d.template === t.id)}
			<li class="[&:not(:first-child)]:shadow-[inset_0_1px_0_var(--line)]">
				<button type="button" class="flex min-h-[56px] w-full items-center gap-3 py-2 text-left" aria-pressed={picked} onclick={() => toggle(t.id)}>
					<SessionIcon icon={t.icon} name={t.name} size={32} />
					<span class="min-w-0 flex-1 truncate text-[15px] font-bold">{t.name}</span>
					<span
						class="flex size-6 shrink-0 items-center justify-center rounded-lg {picked ? 'bg-ink text-ground' : 'shadow-[inset_0_0_0_2px_var(--ink-3)]'}"
					>
						{#if picked}<Icon name="check" size="1rem" />{/if}
					</span>
				</button>
			</li>
		{/each}
	</ul>
	<div class="mt-3.5"><Button onclick={() => (open = false)}>Done</Button></div>
</Sheet>
