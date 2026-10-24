<script lang="ts">
	import type { Origin } from './exercise';
	import { haptic } from './haptics';
	import Icon from './Icon.svelte';
	import Segmented from './Segmented.svelte';
	import Sheet from './Sheet.svelte';
	import { plural } from './text';

	// The filter button beside the search field and the sheet it opens. '' keeps
	// every row. shown is how many rows the filters leave, for the sheet's button.
	let {
		origin = $bindable(),
		source = $bindable(),
		tag = $bindable(),
		sources,
		tags,
		shown,
		noun
	}: { origin: string; source: string; tag: string; sources: string[]; tags: string[]; shown: number; noun: string } = $props();

	let open = $state(false);
	let page = $state<'source' | 'tag' | null>(null);

	const on = $derived([origin, source, tag].filter(Boolean).length);

	const origins: [Origin | '', string][] = [
		['', 'Any'],
		['builtin', 'Built-in'],
		['private', 'Private'],
		['yours', 'Yours']
	];

	const pages = {
		source: { title: 'Source', options: () => sources },
		tag: { title: 'Labels', options: () => tags }
	};

	function pick(v: string) {
		if (page === 'source') source = v;
		else tag = v;
		page = null;
	}
</script>

<button
	type="button"
	aria-label={on ? `Filter, ${on} on` : 'Filter'}
	class="relative flex size-12 shrink-0 items-center justify-center rounded-full {on ? 'bg-ink text-ground' : 'bg-well text-ink dark:bg-surface'}"
	onclick={() => {
		page = null;
		open = true;
	}}
	use:haptic
>
	<Icon name="sliders-horizontal" size="20px" stroke={2.2} />
	{#if on}
		<span class="absolute -top-0.5 -right-0.5 flex size-[19px] items-center justify-center rounded-full bg-tint text-[11px] font-extrabold text-on-tint shadow-[0_0_0_2px_var(--ground)]" aria-hidden="true">{on}</span>
	{/if}
</button>

<Sheet bind:open title={page ? pages[page].title : 'Filter'} onback={page ? () => (page = null) : undefined}>
	{#if page}
		{@const current = page === 'source' ? source : tag}
		<ul class="rounded-3xl bg-surface py-1 shadow-card">
			{#each ['', ...pages[page].options()] as v (v)}
				<li class="list-row">
					<button type="button" class="flex min-h-[50px] w-full items-center gap-3 px-4 py-2 text-left" onclick={() => pick(v)}>
						<span class="min-w-0 flex-1 text-[15px] font-bold">{v || 'Any'}</span>
						{#if v === current}<Icon name="check" size="18px" stroke={2.6} />{/if}
					</button>
				</li>
			{/each}
		</ul>
	{:else}
		<h3 class="mb-2 px-1 text-xs font-bold tracking-wide text-ink-2 uppercase">Origin</h3>
		<Segmented label="Origin" items={origins.map(([v, label]) => ({ label, on: origin === v, onclick: () => (origin = v) }))} />
		<ul class="mt-4 rounded-3xl bg-surface py-1 shadow-card">
			{#each [['source', source], ['tag', tag]] as const as [key, value] (key)}
				{#if value || pages[key].options().length}
					<li class="list-row">
						<button type="button" class="flex min-h-14 w-full items-center gap-3 py-2 pr-3 pl-4 text-left" onclick={() => (page = key)}>
							<span class="text-[15px] font-bold">{pages[key].title}</span>
							<span class="min-w-0 flex-1 truncate text-right text-[15px] font-semibold {value ? 'text-ink' : 'text-ink-3'}">{value || 'Any'}</span>
							<span class="flex text-ink-3"><Icon name="chevron-right" size="18px" stroke={2.2} /></span>
						</button>
					</li>
				{/if}
			{/each}
		</ul>
	{/if}
	{#snippet footer()}
		<button type="button" class="flex h-14 w-full items-center justify-center rounded-full bg-tint text-[15px] font-extrabold text-on-tint" onclick={() => (open = false)} use:haptic>
			Show {plural(shown, noun)}
		</button>
	{/snippet}
</Sheet>
