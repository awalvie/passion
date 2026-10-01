<script lang="ts">
	import { goto } from '$app/navigation';
	import { untrack, type Snippet } from 'svelte';
	import { describe, request } from './api';
	import Button from './Button.svelte';
	import { addDays, weekday } from './dates';
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

	const names = $derived(new Map(templates.map((t) => [t.id, t.name])));
	const length = $derived(
		draft.starts && draft.ends ? Math.round((Date.parse(draft.ends) - Date.parse(draft.starts)) / 86_400_000) + 1 : 0
	);
	const maxBlock = $derived(Math.max(1, Math.min(28, length)));
	const blockDays = $derived(Array.from({ length: Math.max(0, Math.min(draft.block_days, 28)) }, (_, i) => i + 1));

	const fields: Record<string, string> = { name: 'The name', starts: 'The start', ends: 'The end', block_days: 'The block' };
	const label = (f: string) => (f.startsWith('days') ? 'A session' : (fields[f] ?? f));

	function add(day: number, template: string) {
		if (template && !draft.days.some((d) => d.day === day && d.template === template)) {
			draft.days.push({ day, template });
		}
	}

	async function save() {
		busy = true;
		error = '';
		try {
			const { id, ...body } = $state.snapshot(draft);
			body.days = body.days.filter((d) => d.day <= body.block_days);
			const saved = await request<{ left_out: unknown[] }>('PUT', `/api/v1/cycles/${id}`, body);
			if (!saved.left_out.length) return await goto('/plan');
			leftOut = saved.left_out.length;
		} catch (e) {
			error = describe(e, label);
		} finally {
			busy = false;
		}
	}
</script>

<form
	class="flex flex-col gap-5 px-4 pt-2 pb-[calc(env(safe-area-inset-bottom)+5rem)]"
	onsubmit={(e) => {
		e.preventDefault();
		save();
	}}
>
	<label class="flex flex-col gap-1 text-sm text-ink-2">
		Name
		<input class="input" bind:value={draft.name} required maxlength="200" placeholder="Autumn power" />
	</label>

	<div class="grid grid-cols-2 gap-3">
		<label class="flex flex-col gap-1 text-sm text-ink-2">
			Starts
			<input class="input" type="date" bind:value={draft.starts} required />
		</label>
		<label class="flex flex-col gap-1 text-sm text-ink-2">
			Ends
			<input class="input" type="date" bind:value={draft.ends} min={draft.starts} required />
		</label>
	</div>

	<label class="flex flex-col gap-1 text-sm text-ink-2">
		Repeats every
		<span class="flex items-center gap-2 text-base text-ink">
			<input class="input w-20" type="number" inputmode="numeric" min="1" max={maxBlock} bind:value={draft.block_days} required />
			days
		</span>
	</label>

	<section class="flex flex-col gap-2">
		<h2 class="px-1 text-sm font-semibold text-ink-2">Sessions</h2>
		<ul class="overflow-hidden rounded-2xl bg-surface shadow-sm">
			{#each blockDays as day (day)}
				<li class="flex flex-col gap-2 border-line px-4 py-3 [&:not(:first-child)]:border-t">
					<span class="text-sm font-semibold text-ink-2">
						Day {day}{draft.starts ? ` · ${weekday(addDays(draft.starts, day - 1))}` : ''}
					</span>
					{#each draft.days.filter((d) => d.day === day) as d (d.template)}
						{@const name = names.get(d.template) ?? 'Retired session'}
						<span class="flex items-center gap-2">
							<span class="min-w-0 flex-1 truncate text-base">{name}</span>
							<button
								type="button"
								class="h-11 shrink-0 px-2 text-base font-semibold text-bad"
								aria-label="Remove {name} from day {day}"
								onclick={() => (draft.days = draft.days.filter((x) => x !== d))}
							>
								Remove
							</button>
						</span>
					{/each}
					<select
						class="input"
						aria-label="Add a session on day {day}"
						value=""
						onchange={(e) => {
							add(day, e.currentTarget.value);
							e.currentTarget.value = '';
						}}
					>
						<option value="">Add a session…</option>
						{#each templates as t (t.id)}
							<option value={t.id}>{t.name}</option>
						{/each}
					</select>
				</li>
			{/each}
		</ul>
	</section>

	<FormError message={error} />
	{#if leftOut}
		<p class="rounded-2xl bg-surface p-4 text-base shadow-sm">
			Saved. {leftOut === 1 ? '1 day' : `${leftOut} days`} already held that session, so the cycle left
			{leftOut === 1 ? 'it as it was' : 'them as they were'}.
			<a href="/plan" class="font-semibold text-tint">Back to the plan</a>
		</p>
	{/if}
	<Button type="submit" disabled={busy}>Save cycle</Button>
	{@render children?.()}
</form>
