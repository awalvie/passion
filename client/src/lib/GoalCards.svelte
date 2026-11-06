<script lang="ts">
	import { invalidateAll } from '$app/navigation';
	import { describe, request } from './api';
	import Checkbox from './Checkbox.svelte';
	import FormError from './FormError.svelte';
	import { current, goalPeriod, ticked, type PeriodGoal } from './goals';
	import Icon from './Icon.svelte';
	import PeriodGoalSheet from './PeriodGoalSheet.svelte';

	// This week's goals. The other periods live in Plan's goals.
	let { goals, today }: { goals: PeriodGoal[]; today: string } = $props();

	const rows = $derived(current(goals, today).filter((g) => g.period === 'week'));

	let open = $state(false);
	let editing = $state<PeriodGoal | null>(null);
	function edit(g: PeriodGoal | null) {
		editing = g;
		open = true;
	}

	// One tick at a time. A tap meanwhile puts its box back.
	let busy = false;
	let error = $state('');
	async function tick(g: PeriodGoal, box: HTMLInputElement) {
		if (busy) {
			box.checked = !box.checked;
			return;
		}
		busy = true;
		error = '';
		try {
			await request(box.checked ? 'PUT' : 'DELETE', `/api/v1/goals/${g.id}/ticks/${goalPeriod(g, today).from}`);
		} catch (e) {
			error = describe(e, (f) => f);
		}
		await invalidateAll();
		busy = false;
	}
</script>

<a href="/plan?view=goals" class="mx-4 mt-6 flex h-11 items-center gap-1 px-1 text-xl font-bold lg:mt-3.5 lg:text-[15px]"
	>Goals this week<span class="text-ink-3"><Icon name="chevron-right" size="1.125rem" stroke={2.4} /></span></a
>
<section class="mx-4 mt-2 rounded-3xl bg-surface px-[18px] py-1 shadow-card">
	<FormError message={error} />
	<ul>
		{#each rows as g (g.id)}
			{@const done = ticked(g, today)}
			<li class="list-row flex min-h-[52px] items-center gap-1">
				<label class="-ml-2.5 flex size-11 shrink-0 cursor-pointer items-center justify-center">
					<Checkbox checked={done} label={g.text} onchange={(e) => tick(g, e.currentTarget)} />
				</label>
				<button type="button" class="flex min-h-11 min-w-0 flex-1 items-center py-1.5 text-left" onclick={() => edit(g)}>
					<span class="text-[15px] font-semibold break-words {done ? 'text-ink-2 line-through' : ''}">{g.text}</span>
				</button>
			</li>
		{/each}
		<li class="list-row">
			<button type="button" class="flex min-h-[52px] w-full items-center gap-3 text-[15px] font-semibold text-ink-2" onclick={() => edit(null)}>
				<span class="flex size-6 shrink-0 items-center justify-center text-ink-3"><Icon name="plus" size="1.125rem" /></span>
				{rows.length ? 'Add goal' : 'Add a goal for this week'}
			</button>
		</li>
	</ul>
</section>

<PeriodGoalSheet bind:open {editing} span={null} {goals} {today} />
