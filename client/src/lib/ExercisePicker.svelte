<script lang="ts">
	import { kindOf, type Exercise } from '$lib/exercise';

	let {
		exercises,
		id,
		pick
	}: { exercises: Exercise[]; id: string; pick: (e: Exercise) => void } = $props();

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
	<label class="text-xs font-medium" for={id}>Add exercise</label>
	<input
		{id}
		type="search"
		class="mt-1 w-full input text-sm"
		placeholder="Search the library…"
		autocomplete="off"
		bind:value={q}
		onkeydown={key}
	/>
	{#if q.trim()}
		<div class="mt-1 rounded-md border overflow-hidden" style="background:var(--panel);border-color:var(--border)">
			{#each matches as e (e.id)}
				<button
					type="button"
					class="flex w-full items-baseline justify-between gap-3 px-3 py-2 text-left text-sm hover:bg-[var(--card-muted)]"
					onclick={() => choose(e)}
				>
					<span class="min-w-0 truncate">{e.name}</span>
					<span class="shrink-0 text-[11px] muted">{kindOf(e.kind).label}</span>
				</button>
			{:else}
				<p class="px-3 py-2 text-xs muted">
					Nothing matches.
					<a class="link underline" href="/exercises/new" target="_blank" rel="noopener">Add it to the library</a>
					in a new tab, then come back.
				</p>
			{/each}
		</div>
	{/if}
</div>
