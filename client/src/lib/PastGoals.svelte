<script lang="ts">
	import { invalidateAll } from '$app/navigation';
	import { describe, request } from './api';
	import Checkbox from './Checkbox.svelte';
	import FormError from './FormError.svelte';
	import type { PastPeriod } from './goals';

	// A period that has ended, with a box for each goal that counted in it. A
	// goal can still be ticked there, unless it was removed.
	let { period }: { period: PastPeriod } = $props();

	let busy = false;
	let error = $state('');
	async function tick(id: string, box: HTMLInputElement) {
		if (busy) {
			box.checked = !box.checked;
			return;
		}
		busy = true;
		error = '';
		try {
			await request(box.checked ? 'PUT' : 'DELETE', `/api/v1/goals/${id}/ticks/${period.from}`);
		} catch (e) {
			error = describe(e, (f) => f);
		}
		await invalidateAll();
		busy = false;
	}
</script>

<div class="flex flex-col gap-2">
	<ul class="rounded-3xl bg-surface px-[18px] py-1 shadow-card">
		{#each period.goals as g (g.id)}
			<li class="flex min-h-[52px] items-center gap-1 list-row">
				<label class="-ml-2.5 flex size-11 shrink-0 cursor-pointer items-center justify-center has-[:disabled]:cursor-default">
					<Checkbox checked={g.done} label={g.text} disabled={g.removed} onchange={(e) => tick(g.id, e.currentTarget)} />
				</label>
				<span class="min-w-0 flex-1 py-1.5 text-[15px] font-bold break-words {g.done ? 'text-ink-2 line-through' : g.removed ? 'text-ink-2' : ''}">{g.text}</span>
				{#if g.removed}<span class="shrink-0 text-xs font-semibold text-ink-2">Removed</span>{/if}
			</li>
		{/each}
	</ul>
	<FormError message={error} />
	<p class="px-1 text-xs font-semibold text-ink-2">You can still tick a goal you forgot.</p>
</div>
