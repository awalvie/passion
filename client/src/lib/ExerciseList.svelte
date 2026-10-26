<script lang="ts">
	import type { Snippet } from 'svelte';
	import { kindOf, summary, type Exercise } from './exercise';
	import Icon from './Icon.svelte';

	// The library to add from, with a search. Enter adds the first match.
	// inside names where an exercise already is, such as "In Pull". children
	// sit between the search and the list. onnew makes a missing exercise,
	// named as searched.
	let {
		exercises,
		pick,
		inside,
		input = $bindable(),
		children,
		onnew
	}: {
		exercises: Exercise[];
		pick: (e: Exercise) => void;
		inside: (e: Exercise) => string;
		input?: HTMLInputElement;
		children?: Snippet;
		onnew: (name: string) => void;
	} = $props();

	let q = $state('');
	const matches = $derived.by(() => {
		const needle = q.trim().toLowerCase();
		return needle ? exercises.filter((e) => e.name.toLowerCase().includes(needle)) : exercises;
	});

	function key(e: KeyboardEvent) {
		if (e.key === 'Enter') {
			e.preventDefault();
			if (q.trim() && matches.length) {
				pick(matches[0]);
				q = '';
			}
		}
		if (e.key === 'Escape') q = '';
	}
</script>

<div class="flex flex-col gap-3">
	<div class="relative">
		<span class="pointer-events-none absolute top-1/2 left-4 -translate-y-1/2 text-ink-2"><Icon name="search" size="1.125rem" /></span>
		<input
			bind:this={input}
			type="search"
			enterkeyhint="search"
			class="input h-12 pl-11"
			placeholder="Search exercises…"
			aria-label="Search exercises"
			autocomplete="off"
			bind:value={q}
			onkeydown={key}
		/>
	</div>
	{@render children?.()}
	<ul>
		{#each matches as e (e.id)}
			{@const where = inside(e)}
			<li class="list-row flex min-h-16 items-center gap-3 py-2">
				<span class="min-w-0 flex-1">
					<span class="block truncate text-[15px] font-bold">{e.name}</span>
					<span class="block truncate text-xs font-semibold text-ink-2">
						{[kindOf(e.kind).label, summary(e)].filter(Boolean).join(' · ')}{#if where}{' · '}<b class="text-ink">{where}</b>{/if}
					</span>
				</span>
				<button type="button" class="flex size-11 shrink-0 items-center justify-center rounded-full bg-well text-ink" aria-label="Add {e.name}" onclick={() => pick(e)}>
					<Icon name="plus" size="1.25rem" />
				</button>
			</li>
		{:else}
			<li class="mt-1 rounded-2xl bg-well px-4 py-4 text-center text-[15px] font-semibold text-ink-2">
				{q.trim() ? `Nothing matches “${q.trim()}”.` : 'No exercises yet.'}
			</li>
		{/each}
	</ul>
	<div class="flex items-center justify-between gap-3 pt-1">
		<span class="text-xs font-semibold text-ink-2">Not in the library?</span>
		<button type="button" class="flex h-11 items-center gap-1.5 rounded-full bg-well px-4 text-[15px] font-bold text-ink" onclick={() => onnew(q.trim())}>
			<Icon name="plus" />New exercise
		</button>
	</div>
</div>
