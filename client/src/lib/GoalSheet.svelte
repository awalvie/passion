<script lang="ts">
	import Button from './Button.svelte';
	import type { Goal } from './plan';
	import Sheet from './Sheet.svelte';

	// index -1 adds a goal. onsave gets the whole new list; the page saves it.
	let {
		open = $bindable(false),
		goals,
		index,
		onsave
	}: { open?: boolean; goals: Goal[]; index: number; onsave: (goals: Goal[]) => void } = $props();

	const blank: Goal = { text: '', done: false, before: '', after: '', how: '' };
	let draft = $state<Goal>({ ...blank });

	// Each opening starts from the goal as it stands.
	$effect(() => {
		if (open) draft = { ...(goals[index] ?? blank) };
	});

	function save(e: SubmitEvent) {
		e.preventDefault();
		onsave(index < 0 ? [...goals, draft] : goals.map((g, i) => (i === index ? draft : g)));
		open = false;
	}

	function remove() {
		onsave(goals.filter((_, i) => i !== index));
		open = false;
	}

	const fields = [
		['before', 'Before', 'Where you start, for example: hang 3 s on the 10 mm edge'],
		['after', 'After', 'Where you want to end'],
		['how', 'How', 'What you will do, for example: a hangboard session once a week']
	] as const;
</script>

<Sheet bind:open title={index < 0 ? 'New goal' : 'Goal'}>
	<form class="flex flex-col gap-3.5" onsubmit={save}>
		<div class="flex flex-col gap-3.5 rounded-3xl bg-surface p-[18px] shadow-card">
			<label class="flex flex-col gap-1.5 text-xs font-semibold text-ink-2">
				Goal
				<input class="input h-12 px-4" bind:value={draft.text} required maxlength="200" placeholder="Improve finger strength" />
			</label>
			{#each fields as [key, label, hint] (key)}
				<label class="flex flex-col gap-1.5 text-xs font-semibold text-ink-2">
					{label}
					<textarea
						class="input min-h-12 resize-none px-4 py-3 [field-sizing:content] placeholder:text-ink-3"
						bind:value={draft[key]}
						maxlength="500"
						rows="1"
						placeholder={hint}
					></textarea>
				</label>
			{/each}
		</div>
		<div class="grid grid-cols-2 items-center gap-2">
			<Button variant="secondary" onclick={() => (open = false)}>Cancel</Button>
			<Button type="submit">Save</Button>
		</div>
		{#if index >= 0}
			<Button variant="danger" onclick={remove}>Remove goal</Button>
		{/if}
	</form>
</Sheet>
