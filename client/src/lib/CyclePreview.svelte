<script lang="ts">
	import { gridWeeks } from './dates';
	import SessionIcon from './SessionIcon.svelte';
	import type { SessionTemplate } from './template';

	let {
		starts,
		ends,
		planned,
		templates
	}: { starts: string; ends: string; planned: { date: string; template: string }[]; templates: SessionTemplate[] } = $props();

	const weeks = $derived(gridWeeks(starts, ends));
	const byId = $derived(new Map(templates.map((t) => [t.id, t])));
	const byDate = $derived.by(() => {
		const m = new Map<string, string[]>();
		for (const p of planned) m.set(p.date, [...(m.get(p.date) ?? []), p.template]);
		return m;
	});
</script>

<section class="rounded-3xl bg-surface px-2.5 pt-3.5 pb-2.5 shadow-card" aria-label="Calendar">
	<ol class="grid grid-cols-7 text-center text-xs font-bold text-ink-2" aria-hidden="true">
		{#each ['M', 'T', 'W', 'T', 'F', 'S', 'S'] as d, i (i)}
			<li>{d}</li>
		{/each}
	</ol>
	<div class="mt-2 flex flex-col gap-1.5">
		{#each weeks as week (week[0])}
			<div class="grid grid-cols-7 rounded-2xl bg-well">
				{#each week as date (date)}
					{@const inside = date >= starts && date <= ends}
					<div class="flex min-h-[52px] flex-col items-center gap-1 pt-1.5 pb-2">
						<span class="text-xs font-bold {inside ? 'text-ink-2' : 'text-ink-3/50'}">{Number(date.slice(8))}</span>
						{#each byDate.get(date) ?? [] as id (id)}
							{@const t = byId.get(id)}
							<SessionIcon icon={t?.icon ?? null} name={t?.name ?? '?'} size={22} />
						{/each}
					</div>
				{/each}
			</div>
		{/each}
	</div>
</section>
