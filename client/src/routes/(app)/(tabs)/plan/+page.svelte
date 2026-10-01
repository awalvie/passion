<script lang="ts">
	import { invalidateAll } from '$app/navigation';
	import { describe, request } from '$lib/api';
	import Button from '$lib/Button.svelte';
	import FormError from '$lib/FormError.svelte';
	import type { ScheduledDay } from '$lib/plan';

	let { data } = $props();

	const labels: Record<string, string> = { local_date: 'That day', template: 'That session' };

	let adding = $state(false);
	let template = $state('');
	let date = $state('');
	let busy = $state(false);
	let error = $state('');

	function openAdd() {
		adding = true;
		template = data.templates[0]?.id ?? '';
		date = data.today;
		error = '';
	}

	async function add() {
		busy = true;
		error = '';
		try {
			await request('POST', '/api/v1/scheduled-sessions', { template, local_date: date });
			await invalidateAll();
			adding = false;
		} catch (e) {
			error = describe(e, (f) => labels[f] ?? f);
		} finally {
			busy = false;
		}
	}

	const dates = $derived.by(() => {
		const out: { date: string; days: ScheduledDay[] }[] = [];
		for (const d of data.days) {
			if (out.at(-1)?.date !== d.local_date) out.push({ date: d.local_date, days: [] });
			out.at(-1)!.days.push(d);
		}
		return out;
	});

	const cycleNames = $derived(new Map(data.cycles.map((c) => [c.id, c.name])));

	function heading(date: string) {
		if (date === data.today) return 'Today';
		return new Date(`${date}T00:00:00Z`).toLocaleDateString(undefined, {
			weekday: 'short',
			day: 'numeric',
			month: 'short',
			timeZone: 'UTC'
		});
	}

	const statuses = { done: 'Done', started: 'Started', missed: 'Missed', planned: '' };
</script>

<svelte:head><title>Plan</title></svelte:head>

<div class="flex flex-col gap-4 px-4 pt-[calc(env(safe-area-inset-top)+0.75rem)]">
	<h1 class="text-3xl font-bold">Plan</h1>

	{#if data.offline}
		<p class="rounded-2xl bg-surface p-4 text-base text-ink-2 shadow-sm">No signal. The plan needs one to load.</p>
	{:else}
		{#each dates as g (g.date)}
			<section class="flex flex-col gap-2">
				<h2 class="px-1 text-sm font-semibold {g.date === data.today ? 'text-tint' : 'text-ink-2'}">{heading(g.date)}</h2>
				<ul class="overflow-hidden rounded-2xl bg-surface shadow-sm">
					{#each g.days as d (d.id)}
						<li class="flex items-center gap-3 border-line px-4 py-3 [&:not(:first-child)]:border-t">
							<span class="min-w-0 flex-1">
								<span class="block truncate text-base">{d.template_name}</span>
								<span class="block truncate text-sm text-ink-2">
									{d.cycle ? (cycleNames.get(d.cycle) ?? 'Cycle') : 'One-off'}
								</span>
							</span>
							<span class="shrink-0 text-sm {d.status === 'missed' ? 'text-bad' : 'text-ink-2'}">{statuses[d.status]}</span>
						</li>
					{/each}
				</ul>
			</section>
		{:else}
			<p class="rounded-2xl bg-surface p-4 text-base text-ink-2 shadow-sm">Nothing planned for the next four weeks.</p>
		{/each}

		{#if adding}
			<form
				class="flex flex-col gap-3 rounded-2xl bg-surface p-4 shadow-sm"
				onsubmit={(e) => {
					e.preventDefault();
					add();
				}}
			>
				<label class="flex flex-col gap-1 text-sm text-ink-2">
					Session
					<select class="input" bind:value={template} required>
						{#each data.templates as t (t.id)}
							<option value={t.id}>{t.name}</option>
						{/each}
					</select>
				</label>
				<label class="flex flex-col gap-1 text-sm text-ink-2">
					Day
					<input class="input" type="date" bind:value={date} required />
				</label>
				<FormError message={error} />
				<div class="grid grid-cols-2 gap-2">
					<Button variant="secondary" onclick={() => (adding = false)}>Cancel</Button>
					<Button type="submit" disabled={busy}>Add</Button>
				</div>
			</form>
		{:else}
			<Button variant="secondary" onclick={openAdd}>Add a session</Button>
		{/if}
	{/if}
</div>
