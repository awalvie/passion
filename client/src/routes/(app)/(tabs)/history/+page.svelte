<script lang="ts">
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

	function when(r: RunSummary) {
		const d = new Date(`${r.local_date}T12:00:00`);
		return {
			weekday: d.toLocaleDateString(undefined, { weekday: 'short' }),
			day: d.getDate()
		};
	}

	function facts(r: RunSummary) {
		const minutes = r.elapsed_seconds === null ? '' : `${Math.round(r.elapsed_seconds / 60)} min`;
		return [minutes, r.place].filter(Boolean).join(' · ');
	}
</script>

<svelte:head><title>History</title></svelte:head>

<div class="flex flex-col gap-3.5 px-4 pt-[calc(env(safe-area-inset-top)+0.75rem)]">
	<h1 class="text-[32px] leading-[1.1] font-extrabold tracking-[-0.02em]">History</h1>

	{#each months as m (m.label)}
		<section class="rounded-3xl bg-surface px-[18px] pt-4 pb-2 shadow-card">
			<h2 class="mb-1 text-[15px] font-bold">{m.label}</h2>
			<ul>
				{#each m.runs as r (r.id)}
					{@const w = when(r)}
					<li class="[&:not(:first-child)]:shadow-[inset_0_1px_0_var(--line)]">
						<a href="/history/{r.id}" class="flex min-h-[58px] items-center gap-3.5 py-2">
							<span class="flex w-8 shrink-0 flex-col">
								<span class="text-xs font-semibold text-ink-2">{w.weekday}</span>
								<b class="text-xl leading-[1.05] font-bold">{w.day}</b>
							</span>
							<span class="min-w-0 flex-1">
								<span class="block truncate text-[15px] font-bold">{r.name}</span>
								{#if facts(r)}
									<span class="mt-0.5 block truncate text-xs font-semibold text-ink-2">{facts(r)}</span>
								{/if}
							</span>
							{#if r.template === null}
								<span class="shrink-0 rounded-xl bg-ink px-2.5 py-[5px] text-xs font-bold text-ground">Open</span>
							{/if}
						</a>
					</li>
				{/each}
			</ul>
		</section>
	{:else}
		<section class="flex flex-col gap-3 rounded-3xl bg-surface p-5 shadow-card">
			<p class="text-xl font-bold tracking-tight">No sessions yet</p>
			<p class="text-[15px] font-semibold text-ink-2">Finished sessions show here.</p>
			<a href="/" class="flex h-12 items-center justify-center rounded-full bg-tint text-[15px] font-bold text-on-tint shadow-tint">Start one</a>
		</section>
	{/each}
</div>
