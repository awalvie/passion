<script lang="ts">
	import Icon from './Icon.svelte';

	// onchange runs after each add or remove, with the new list.
	let {
		entries = $bindable(),
		label,
		onchange
	}: { entries: string[]; label: string; onchange?: (entries: string[]) => void } = $props();

	let text = $state('');

	function set(next: string[]) {
		entries = next;
		onchange?.(next);
	}

	function add() {
		const t = text.trim();
		if (!t) return;
		set([...entries, t]);
		text = '';
	}
</script>

<ul>
	{#each entries as entry, i (i)}
		<li class="flex min-h-[52px] items-center gap-3 py-2 [&:not(:first-child)]:shadow-[inset_0_1px_0_var(--line)]">
			<span class="min-w-0 flex-1 text-[15px] font-semibold break-words whitespace-pre-line">{entry}</span>
			<button
				type="button"
				class="flex size-9 shrink-0 items-center justify-center rounded-full text-ink-3"
				aria-label="Remove {entry}"
				onclick={() => set(entries.filter((_, j) => j !== i))}
			>
				<Icon name="x" size="1rem" />
			</button>
		</li>
	{/each}
	<li class="flex min-h-[52px] items-center gap-3 py-2 [&:not(:first-child)]:shadow-[inset_0_1px_0_var(--line)]">
		<span class="text-ink-3"><Icon name="plus" size="1.125rem" /></span>
		<input
			class="min-w-0 flex-1 bg-transparent text-[15px] font-bold placeholder:text-ink-3 focus:outline-none"
			placeholder="Add entry"
			aria-label={label}
			bind:value={text}
			onkeydown={(e) => {
				if (e.key === 'Enter') {
					e.preventDefault();
					add();
				}
			}}
			onblur={add}
		/>
	</li>
</ul>
