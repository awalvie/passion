<script lang="ts">
	import type { ScheduledDay } from '$lib/plan';

	let { data } = $props();

	const dates = $derived.by(() => {
		const out: { date: string; days: ScheduledDay[] }[] = [];
		for (const d of data.days) {
			if (out.at(-1)?.date !== d.local_date) out.push({ date: d.local_date, days: [] });
			out.at(-1)!.days.push(d);
		}
		return out;
	});

	const cycleNames = $derived(new Map(data.cycles.map((c) => [c.id, c.name])));

	function heading(date: string) {
		if (date === data.today) return 'Today';
		return new Date(`${date}T00:00:00Z`).toLocaleDateString(undefined, {
			weekday: 'short',
			day: 'numeric',
			month: 'short',
			timeZone: 'UTC'
		});
	}

	const statuses = { done: 'Done', started: 'Started', missed: 'Missed', planned: '' };
</script>

<svelte:head><title>Plan</title></svelte:head>

<div class="flex flex-col gap-4 px-4 pt-[calc(env(safe-area-inset-top)+0.75rem)]">
	<h1 class="text-3xl font-bold">Plan</h1>

	{#if data.offline}
		<p class="rounded-2xl bg-surface p-4 text-base text-ink-2 shadow-sm">No signal. The plan needs one to load.</p>
	{:else}
		{#each dates as g (g.date)}
			<section class="flex flex-col gap-2">
				<h2 class="px-1 text-sm font-semibold {g.date === data.today ? 'text-tint' : 'text-ink-2'}">{heading(g.date)}</h2>
				<ul class="overflow-hidden rounded-2xl bg-surface shadow-sm">
					{#each g.days as d (d.id)}
						<li class="flex items-center gap-3 border-line px-4 py-3 [&:not(:first-child)]:border-t">
							<span class="min-w-0 flex-1">
								<span class="block truncate text-base">{d.template_name}</span>
								<span class="block truncate text-sm text-ink-2">
									{d.cycle ? (cycleNames.get(d.cycle) ?? 'Cycle') : 'One-off'}
								</span>
							</span>
							<span class="shrink-0 text-sm {d.status === 'missed' ? 'text-bad' : 'text-ink-2'}">{statuses[d.status]}</span>
						</li>
					{/each}
				</ul>
			</section>
		{:else}
			<p class="rounded-2xl bg-surface p-4 text-base text-ink-2 shadow-sm">Nothing planned for the next four weeks.</p>
		{/each}
	{/if}
</div>
