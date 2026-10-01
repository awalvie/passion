<script lang="ts">
	import { goto } from '$app/navigation';
	import { untrack, type Snippet } from 'svelte';
	import { describe, request } from './api';
	import Button from './Button.svelte';
	import BlockDays from './BlockDays.svelte';
	import FormError from './FormError.svelte';
	import type { Cycle } from './plan';
	import type { SessionTemplate } from './template';

	let {
		cycle,
		templates,
		children
	}: { cycle: Cycle; templates: SessionTemplate[]; children?: Snippet } = $props();

	// The form edits a copy, and keeps the id it opened with, so a retry after a
	// lost answer replaces the same cycle.
	let draft = $state(untrack(() => ({ ...cycle, days: cycle.days.map((d) => ({ ...d })) })));

	let busy = $state(false);
	let error = $state('');
	let leftOut = $state(0);

	const length = $derived(
		draft.starts && draft.ends ? Math.round((Date.parse(draft.ends) - Date.parse(draft.starts)) / 86_400_000) + 1 : 0
	);
	const maxBlock = $derived(Math.max(1, Math.min(28, length)));
	// Day 1 of the block, as the server will count it.
	const blockFrom = $derived(draft.starts === cycle.starts ? cycle.block_from : draft.starts);

	const fields: Record<string, string> = { name: 'The name', starts: 'The start', ends: 'The end', block_days: 'The block' };
	const label = (f: string) => (f.startsWith('days') ? 'A session' : (fields[f] ?? f));

	async function save() {
		busy = true;
		error = '';
		try {
			const { id, block_from, ...body } = $state.snapshot(draft);
			body.days = body.days.filter((d) => d.day <= body.block_days);
			const saved = await request<{ left_out: unknown[] }>('PUT', `/api/v1/cycles/${id}`, body);
			if (!saved.left_out.length) return await goto('/plan?view=cycles');
			leftOut = saved.left_out.length;
		} catch (e) {
			error = describe(e, label);
		} finally {
			busy = false;
		}
	}
</script>

<form
	class="flex flex-col gap-3.5 px-4 pt-2 pb-[calc(env(safe-area-inset-bottom)+5rem)]"
	onsubmit={(e) => {
		e.preventDefault();
		save();
	}}
>
	<div class="flex flex-col gap-3.5 rounded-3xl bg-surface p-[18px] shadow-card">
		<label class="flex flex-col gap-1.5 text-xs font-semibold text-ink-2">
			Name
			<input class="input h-12 px-4" bind:value={draft.name} required maxlength="200" placeholder="Autumn power" />
		</label>

		<div class="grid grid-cols-2 gap-3">
			<label class="flex flex-col gap-1.5 text-xs font-semibold text-ink-2">
				Starts
				<input class="input h-12 px-4" type="date" bind:value={draft.starts} required />
			</label>
			<label class="flex flex-col gap-1.5 text-xs font-semibold text-ink-2">
				Ends
				<input class="input h-12 px-4" type="date" bind:value={draft.ends} min={draft.starts} required />
			</label>
		</div>

		<label class="flex flex-col gap-1.5 text-xs font-semibold text-ink-2">
			Repeats every
			<span class="flex items-center gap-2 text-[15px] font-bold text-ink">
				<input class="input h-12 px-4 w-20 text-center" type="number" inputmode="numeric" min="1" max={maxBlock} bind:value={draft.block_days} required />
				days
			</span>
		</label>
	</div>

	<section class="rounded-3xl bg-surface px-[18px] pt-4 pb-2 shadow-card">
		<h2 class="mb-1 text-[15px] font-bold">Sessions</h2>
		<BlockDays bind:days={draft.days} blockDays={draft.block_days} from={blockFrom} {templates} />
	</section>

	<FormError message={error} />
	{#if leftOut}
		<p class="rounded-3xl bg-surface p-[18px] text-[15px] font-semibold shadow-card">
			Saved. {leftOut === 1 ? '1 day' : `${leftOut} days`} already held that session, so the cycle left
			{leftOut === 1 ? 'it as it was' : 'them as they were'}.
			<a href="/plan?view=cycles" class="font-bold text-link underline">Back to the plan</a>
		</p>
	{/if}
	<Button type="submit" disabled={busy}>Save cycle</Button>
	{@render children?.()}
</form>
