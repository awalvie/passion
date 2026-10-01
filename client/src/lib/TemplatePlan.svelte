<script lang="ts">
	import Notes from '$lib/Notes.svelte';
	import { choiceMeta, stepMeta, type Section } from '$lib/template';

	let { sections }: { sections: Section[] } = $props();

	const hairline = '[&:not(:first-child)]:shadow-[inset_0_1px_0_var(--line)]';
	const row = `flex min-h-11 items-center justify-between gap-3 px-4 py-2 text-[15px] font-semibold ${hairline}`;
</script>

<section class="flex flex-col">
	<h3 class="sr-only">Plan</h3>
	{#each sections as section, i (i)}
		<div class="flex items-baseline gap-2.5 px-1 pt-[18px] pb-2">
			<span class="flex size-[22px] shrink-0 items-center justify-center self-center rounded-full bg-ink text-xs font-bold text-ground">{i + 1}</span>
			<span class="min-w-0 text-[15px] font-bold break-words">{section.name}</span>
			<span class="ml-auto shrink-0 text-xs font-semibold text-ink-2">
				{section.items.length} exercise{section.items.length === 1 ? '' : 's'}
			</span>
		</div>
		{#if section.notes}
			<Notes text={section.notes} class="px-1 pb-2 text-xs font-semibold text-ink-2" />
		{/if}
		<ul class="rounded-[20px] bg-surface py-0.5 shadow-card">
			{#each section.items as item, j (j)}
				{#if item.step}
					<li class={row}>
						<span class="min-w-0 break-words">{item.step.name}</span>
						<span class="shrink-0 text-xs font-semibold text-ink-2">{stepMeta(item.step)}</span>
					</li>
				{:else}
					<li class="px-4 pt-2.5 pb-0.5 text-xs font-semibold tracking-[.06em] text-ink-2 uppercase">
						{item.choice.name} · {choiceMeta(item.choice)}
						{#if item.choice.notes}
							<Notes text={item.choice.notes} class="mt-1 font-semibold tracking-normal text-ink-2 normal-case" />
						{/if}
					</li>
					{#each item.choice.options as option, k (k)}
						<li class="flex min-h-11 items-center justify-between gap-3 px-4 py-2 text-[15px] font-semibold {k ? hairline : ''}">
							<span class="min-w-0 break-words">{option.name}</span>
							<span class="shrink-0 text-xs font-semibold text-ink-2">{stepMeta(option)}</span>
						</li>
					{/each}
				{/if}
			{:else}
				<li class="px-4 py-3 text-xs font-semibold text-ink-2">No exercises</li>
			{/each}
		</ul>
	{:else}
		<p class="rounded-3xl bg-surface p-[18px] text-[15px] font-semibold text-ink-2 shadow-card">No sections yet.</p>
	{/each}
</section>
