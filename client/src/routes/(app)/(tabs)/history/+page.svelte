<script lang="ts">
	import Icon from '$lib/Icon.svelte';
	import type { RunSummary } from '$lib/run';

	let { data } = $props();

	const months = $derived.by(() => {
		const out: { label: string; runs: RunSummary[] }[] = [];
		for (const r of data.runs) {
			const label = new Date(`${r.local_date}T12:00:00`).toLocaleDateString(undefined, {
				month: 'long',
				year: 'numeric'
			});
			if (out.at(-1)?.label !== label) out.push({ label, runs: [] });
			out.at(-1)!.runs.push(r);
		}
		return out;
	});

	function day(r: RunSummary) {
		return new Date(`${r.local_date}T12:00:00`).toLocaleDateString(undefined, {
			weekday: 'short',
			day: 'numeric'
		});
	}

	function facts(r: RunSummary) {
		const minutes = r.elapsed_seconds === null ? '' : `${Math.round(r.elapsed_seconds / 60)} min`;
		return [minutes, r.place].filter(Boolean).join(' · ');
	}
</script>

<svelte:head><title>History</title></svelte:head>

<div class="flex flex-col gap-4 px-4 pt-[calc(env(safe-area-inset-top)+0.75rem)]">
	<h1 class="text-3xl font-bold">History</h1>

	{#each months as m (m.label)}
		<section class="flex flex-col gap-2">
			<h2 class="px-1 text-sm font-semibold text-ink-2">{m.label}</h2>
			<ul class="overflow-hidden rounded-2xl bg-surface shadow-sm">
				{#each m.runs as r (r.id)}
					<li class="border-line [&:not(:first-child)]:border-t">
						<a href="/history/{r.id}" class="flex items-center gap-3 px-4 py-3">
							<span class="w-14 shrink-0 text-sm text-ink-2">{day(r)}</span>
							<span class="min-w-0 flex-1">
								<span class="block truncate text-base">{r.name}</span>
								<span class="block truncate text-sm text-ink-2">{facts(r)}</span>
							</span>
							<Icon name="chevron-right" size="1rem" />
						</a>
					</li>
				{/each}
			</ul>
		</section>
	{:else}
		<p class="rounded-2xl bg-surface p-4 text-base text-ink-2 shadow-sm">
			Finished sessions show here.
		</p>
	{/each}
</div>
