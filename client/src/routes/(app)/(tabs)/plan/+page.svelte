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

	let rowError = $state({ id: '', message: '' });

	async function move(d: ScheduledDay, to: string) {
		busy = true;
		rowError = { id: '', message: '' };
		try {
			await request('PUT', `/api/v1/scheduled-sessions/${d.id}`, { local_date: to });
			await invalidateAll();
		} catch (e) {
			rowError = { id: d.id, message: describe(e, (f) => labels[f] ?? f) };
		} finally {
			busy = false;
		}
	}

	async function remove(d: ScheduledDay) {
		const warning = d.cycle
			? ' It comes back if you change the dates, the block or the days of its cycle.'
			: '';
		if (!confirm(`Take ${d.template_name} off this day?${warning}`)) return;
		busy = true;
		rowError = { id: '', message: '' };
		try {
			await request('DELETE', `/api/v1/scheduled-sessions/${d.id}`);
			await invalidateAll();
		} catch (e) {
			rowError = { id: d.id, message: describe(e, (f) => labels[f] ?? f) };
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

	function short(date: string) {
		return new Date(`${date}T00:00:00Z`).toLocaleDateString(undefined, {
			day: 'numeric',
			month: 'short',
			timeZone: 'UTC'
		});
	}

	const statuses ={ done: 'Done', started: 'Started', missed: 'Missed', planned: '' };
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
						{#snippet row()}
							<span class="min-w-0 flex-1">
								<span class="block truncate text-base">{d.template_name}</span>
								<span class="block truncate text-sm text-ink-2">
									{d.cycle ? (cycleNames.get(d.cycle) ?? 'Cycle') : 'One-off'}
								</span>
							</span>
							<span class="shrink-0 text-sm {d.status === 'missed' ? 'text-bad' : 'text-ink-2'}">{statuses[d.status]}</span>
						{/snippet}
						<li class="border-line [&:not(:first-child)]:border-t">
							{#if d.status === 'planned' || d.status === 'missed'}
								<details>
									<summary class="flex cursor-pointer items-center gap-3 px-4 py-3">{@render row()}</summary>
									<form
										class="flex flex-col gap-3 px-4 pb-4"
										onsubmit={(e) => {
											e.preventDefault();
											move(d, String(new FormData(e.currentTarget).get('date')));
										}}
									>
										<div class="flex gap-2">
											<input class="input min-w-0 flex-1" type="date" name="date" value={d.local_date} required aria-label="New day" />
											<button type="submit" class="h-11 shrink-0 rounded-xl px-3 text-base font-semibold text-tint" disabled={busy}>
												Move
											</button>
										</div>
										<FormError message={rowError.id === d.id ? rowError.message : ''} />
										<button type="button" class="h-11 self-start text-base font-semibold text-bad" disabled={busy} onclick={() => remove(d)}>
											Remove
										</button>
									</form>
								</details>
							{:else}
								<div class="flex items-center gap-3 px-4 py-3">{@render row()}</div>
							{/if}
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

		<section class="flex flex-col gap-2 pt-4">
			<h2 class="px-1 text-sm font-semibold text-ink-2">Cycles</h2>
			{#if data.cycles.length}
				<ul class="overflow-hidden rounded-2xl bg-surface shadow-sm">
					{#each data.cycles as c (c.id)}
						<li class="flex flex-col border-line px-4 py-3 [&:not(:first-child)]:border-t">
							<span class="truncate text-base">{c.name}</span>
							<span class="truncate text-sm text-ink-2">{short(c.starts)} – {short(c.ends)}</span>
						</li>
					{/each}
				</ul>
			{:else}
				<p class="rounded-2xl bg-surface p-4 text-base text-ink-2 shadow-sm">
					A cycle repeats a block of sessions over weeks.
				</p>
			{/if}
			<Button variant="secondary" href="/plan/cycles/new">New cycle</Button>
		</section>
	{/if}
</div>
