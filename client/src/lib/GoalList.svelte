<script lang="ts">
	import Icon from './Icon.svelte';
	import type { Cycle } from './plan';

	// onchange runs after each add, tick or remove, with the new list.
	let {
		goals = $bindable(),
		onchange
	}: { goals: Cycle['goals']; onchange?: (goals: Cycle['goals']) => void } = $props();

	let text = $state('');

	function set(next: Cycle['goals']) {
		goals = next;
		onchange?.(next);
	}

	function add() {
		const t = text.trim();
		if (!t) return;
		set([...goals, { text: t, done: false }]);
		text = '';
	}
</script>

<ul class="rounded-3xl bg-surface px-[18px] shadow-card">
	{#each goals as g, i (i)}
		<li class="flex min-h-[52px] items-center gap-3 py-2 [&:not(:first-child)]:shadow-[inset_0_1px_0_var(--line)]">
			<label class="flex min-w-0 flex-1 items-center gap-3 self-stretch">
				<input
					type="checkbox"
					class="size-6 shrink-0 accent-ink"
					checked={g.done}
					onchange={(e) => set(goals.map((x, j) => (j === i ? { ...x, done: e.currentTarget.checked } : x)))}
				/>
				<span class="min-w-0 flex-1 text-[15px] font-bold break-words {g.done ? 'text-ink-3 line-through' : ''}">{g.text}</span>
			</label>
			<button
				type="button"
				class="flex size-11 shrink-0 items-center justify-center rounded-full text-ink-3"
				aria-label="Remove {g.text}"
				onclick={() => set(goals.filter((_, j) => j !== i))}
			>
				<Icon name="x" size="1rem" />
			</button>
		</li>
	{/each}
	<li class="flex min-h-[52px] items-center gap-3 py-2 [&:not(:first-child)]:shadow-[inset_0_1px_0_var(--line)]">
		<span class="flex size-6 shrink-0 items-center justify-center text-ink-3"><Icon name="plus" size="1.125rem" /></span>
		<input
			class="min-w-0 flex-1 bg-transparent text-[15px] font-bold placeholder:text-ink-3 focus:outline-none"
			placeholder="Add goal"
			maxlength="200"
			aria-label="New goal"
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
