<script lang="ts">
	import { kindOf, type Exercise } from '$lib/exercise';

	let {
		exercises,
		id,
		label = 'Add exercise',
		pick
	}: { exercises: Exercise[]; id: string; label?: string; pick: (e: Exercise) => void } = $props();

	let q = $state('');

	const matches = $derived.by(() => {
		const needle = q.trim().toLowerCase();
		return needle ? exercises.filter((e) => e.name.toLowerCase().includes(needle)).slice(0, 8) : [];
	});

	function choose(e: Exercise) {
		pick(e);
		q = '';
	}

	// Enter would otherwise submit the whole editor.
	function key(event: KeyboardEvent) {
		if (event.key === 'Enter') {
			event.preventDefault();
			if (matches.length) choose(matches[0]);
		}
		if (event.key === 'Escape') q = '';
	}
</script>

<div>
	<label class="block text-xs font-semibold text-ink-2" for={id}>{label}</label>
	<input
		{id}
		type="search"
		class="mt-1.5 w-full input"
		placeholder="Search the library…"
		autocomplete="off"
		bind:value={q}
		onkeydown={key}
	/>
	{#if q.trim()}
		<div class="mt-2 overflow-hidden [&>:not(:first-child)]:shadow-[inset_0_1px_0_var(--line)] rounded-2xl bg-surface shadow-card-sm">
			{#each matches as e (e.id)}
				<button
					type="button"
					class="flex min-h-12 w-full items-center justify-between gap-3 px-4 py-2.5 text-left active:bg-well"
					onclick={() => choose(e)}
				>
					<span class="min-w-0 truncate text-[15px] font-bold">{e.name}</span>
					<span class="shrink-0 text-xs font-semibold text-ink-2">{kindOf(e.kind).label}</span>
				</button>
			{:else}
				<p class="px-4 py-3 text-xs font-semibold text-ink-2">
					Nothing matches.
					<a class="font-bold text-link underline" href="/exercises/new" target="_blank" rel="noopener">Add it to the library</a>
					in a new tab, then come back.
				</p>
			{/each}
		</div>
	{/if}
</div>
