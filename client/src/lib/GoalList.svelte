<script lang="ts">
	import Checkbox from './Checkbox.svelte';
	import Icon from './Icon.svelte';
	import type { Goal } from './plan';

	// A tap on a goal, or on Add goal (index -1), asks the page to open its
	// editor; the editor sits at the page's root, outside any form. onchange
	// runs after a tick, with the new list.
	let {
		goals = $bindable(),
		onedit,
		onchange
	}: { goals: Goal[]; onedit: (index: number) => void; onchange?: (goals: Goal[]) => void } = $props();

	function tick(i: number, done: boolean) {
		goals = goals.map((x, j) => (j === i ? { ...x, done } : x));
		onchange?.(goals);
	}
</script>

<ul class="rounded-3xl bg-surface px-[18px] shadow-card">
	{#each goals as g, i (i)}
		<li class="flex min-h-[52px] items-start gap-1 py-1.5 [&:not(:first-child)]:shadow-[inset_0_1px_0_var(--line)]">
			<label class="-ml-2.5 flex size-11 shrink-0 cursor-pointer items-center justify-center">
				<Checkbox checked={g.done} label={g.text} onchange={(e) => tick(i, e.currentTarget.checked)} />
			</label>
			<button type="button" class="flex min-h-11 min-w-0 flex-1 flex-col justify-center gap-0.5 py-1.5 text-left" onclick={() => onedit(i)}>
				<span class="text-[15px] font-bold break-words {g.done ? 'text-ink-3 line-through' : ''}">{g.text}</span>
				{#each [['Before', g.before], ['After', g.after], ['How', g.how]] as [label, value] (label)}
					{#if value}
						<span class="text-xs font-semibold break-words text-ink-2"><b class="text-ink">{label}</b> {value}</span>
					{/if}
				{/each}
			</button>
			<span class="flex h-11 shrink-0 items-center text-ink-3"><Icon name="chevron-right" size="1rem" /></span>
		</li>
	{/each}
	<li class="[&:not(:first-child)]:shadow-[inset_0_1px_0_var(--line)]">
		<button type="button" class="flex min-h-[52px] w-full items-center gap-3 py-2 text-[15px] font-bold text-ink-2" onclick={() => onedit(-1)}>
			<span class="flex size-6 shrink-0 items-center justify-center text-ink-3"><Icon name="plus" size="1.125rem" /></span>
			Add goal
		</button>
	</li>
</ul>
