<script lang="ts">
	import Icon from './Icon.svelte';
	import { choiceMeta, loadTemplates, type Section, type SessionTemplate } from './template';
	import { plural } from './text';

	// The sections of your other sessions, to copy one in. copied holds the
	// ones copied since the builder opened, which show a tick.
	let {
		exclude,
		copy,
		copied
	}: { exclude?: string; copy: (s: Section, key: string) => void; copied: string[] } = $props();

	let templates = $state<SessionTemplate[] | null>(null);
	let failed = $state(false);
	loadTemplates().then(
		(t) => (templates = t),
		() => (failed = true)
	);

	let q = $state('');
	const groups = $derived.by(() => {
		const needle = q.trim().toLowerCase();
		return (templates ?? [])
			.filter((t) => t.id !== exclude)
			.map((t) => ({ t, sections: t.sections.map((s, i) => ({ s, key: `${t.id}:${i}` })).filter(({ s }) => !needle || s.name.toLowerCase().includes(needle)) }))
			.filter((g) => g.sections.length);
	});

	const meta = (s: Section) => (s.items.length === 1 && s.items[0].choice && choiceMeta(s.items[0].choice)) || plural(s.items.length, 'exercise');
</script>

<div class="flex flex-col gap-3">
	<div class="relative">
		<span class="pointer-events-none absolute top-1/2 left-4 -translate-y-1/2 text-ink-2"><Icon name="search" size="1.125rem" /></span>
		<!-- The list sits inside the builder's form, where Enter would save. -->
		<input
			type="search"
			class="input h-12 pl-11"
			placeholder="Search sections…"
			aria-label="Search sections"
			autocomplete="off"
			bind:value={q}
			onkeydown={(e) => e.key === 'Enter' && e.preventDefault()}
		/>
	</div>
	{#if failed}
		<p class="px-1 text-[15px] font-semibold text-ink-2">No signal. Your sessions need one to load.</p>
	{:else if !templates}
		<p class="px-1 text-[15px] font-semibold text-ink-2">Loading…</p>
	{:else}
		{#each groups as { t, sections } (t.id)}
			<section>
				<div class="flex items-baseline justify-between gap-3 pt-2">
					<h3 class="truncate text-[15px] font-bold">{t.name}</h3>
					{#if t.shipped}<span class="shrink-0 text-xs font-semibold text-ink-2">Built in</span>{/if}
				</div>
				<ul>
					{#each sections as { s, key } (key)}
						{@const done = copied.includes(key)}
						<li class="list-row flex min-h-16 items-center gap-3 py-2">
							<span class="min-w-0 flex-1">
								<span class="block truncate text-[15px] font-bold">{s.name}</span>
								<span class="block truncate text-xs font-semibold text-ink-2">{meta(s)}</span>
							</span>
							<button
								type="button"
								class="flex size-11 shrink-0 items-center justify-center rounded-full {done ? 'bg-ink text-ground' : 'bg-well text-ink'}"
								aria-label="Copy {s.name} from {t.name}"
								onclick={() => copy(s, key)}
							>
								<Icon name={done ? 'check' : 'copy'} size="1.125rem" />
							</button>
						</li>
					{/each}
				</ul>
			</section>
		{:else}
			<p class="mt-1 rounded-2xl bg-well px-4 py-4 text-center text-[15px] font-semibold text-ink-2">
				{q.trim() ? `Nothing matches “${q.trim()}”.` : 'Your other sessions have no sections yet.'}
			</p>
		{/each}
	{/if}
</div>
