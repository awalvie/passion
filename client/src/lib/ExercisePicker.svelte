<script lang="ts">
	import { kindOf, type Exercise } from '$lib/exercise';

	let {
		exercises,
		id,
		label,
		pick
	}: { exercises: Exercise[]; id: string; label: string; pick: (e: Exercise) => void } = $props();

	let q = $state('');
	let browsing = $state(false);

	const matches = $derived.by(() => {
		const needle = q.trim().toLowerCase();
		return needle ? exercises.filter((e) => e.name.toLowerCase().includes(needle)) : browsing ? exercises : [];
	});

	// Enter would otherwise submit the whole editor.
	function key(event: KeyboardEvent) {
		if (event.key === 'Enter') {
			event.preventDefault();
			if (q.trim() && matches.length) pick(matches[0]);
		}
		if (event.key === 'Escape') {
			q = '';
			browsing = false;
		}
	}
</script>

<div>
	<label class="block text-xs font-semibold text-ink-2" for={id}>{label}</label>
	<input
		enterkeyhint="search"
		{id}
		type="search"
		class="mt-1.5 input"
		placeholder="Search the library…"
		autocomplete="off"
		bind:value={q}
		onkeydown={key}
	/>
	{#if !q.trim()}
		<button type="button" class="flex min-h-11 items-center text-xs font-bold text-ink" aria-expanded={browsing} onclick={() => (browsing = !browsing)}>
			{browsing ? 'Hide the library' : 'Browse the library'}
		</button>
	{/if}
	{#if q.trim() || browsing}
		<div
			class="mt-2 max-h-80 overflow-y-auto overscroll-contain [&>:not(:first-child)]:shadow-[inset_0_1px_0_var(--line)] rounded-2xl bg-surface shadow-card-sm"
		>
			{#each matches as e (e.id)}
				<button
					type="button"
					class="flex min-h-12 w-full items-center justify-between gap-3 px-4 py-2.5 text-left active:bg-well"
					onclick={() => pick(e)}
				>
					<span class="min-w-0 truncate text-[15px] font-bold">{e.name}</span>
					<span class="shrink-0 text-xs font-semibold text-ink-2">{kindOf(e.kind).label}</span>
				</button>
			{:else}
				<p class="px-4 py-3 text-xs font-semibold text-ink-2">
					Nothing matches.
					<a class="font-bold text-ink underline" href="/exercises/new" target="_blank" rel="noopener">Add it to the library</a>
					in a new tab, then come back.
				</p>
			{/each}
		</div>
	{/if}
</div>
