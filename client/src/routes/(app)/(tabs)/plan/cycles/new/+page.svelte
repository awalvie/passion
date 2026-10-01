<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { untrack } from 'svelte';
	import { describe } from '$lib/api';
	import BlockDays from '$lib/BlockDays.svelte';
	import Button from '$lib/Button.svelte';
	import { addDays, daysBetween, formatDate } from '$lib/dates';
	import EntryList from '$lib/EntryList.svelte';
	import FormError from '$lib/FormError.svelte';
	import GoalList from '$lib/GoalList.svelte';
	import NavBar from '$lib/NavBar.svelte';
	import { saveCycle } from '$lib/plan';

	let { data } = $props();

	// The step lives in the address, so the phone's back gesture steps back.
	// The draft keeps the id it opened with, so a retry makes one cycle.
	const step = $derived(Math.min(3, Math.max(1, Number(page.url.searchParams.get('step')) || 1)));
	let draft = $state(untrack(() => structuredClone($state.snapshot(data.cycle))));
	let busy = $state(false);
	let error = $state('');
	let leftOut = $state(0);

	const length = $derived(daysBetween(draft.starts, draft.ends) + 1);
	const fields: Record<string, string> = { name: 'The name', starts: 'The start', ends: 'The end', block_days: 'The block' };
	const titles = ['Name and dates', 'What is it for?', 'What repeats?'];
	const hints = ['The block comes next.', 'All optional. You can add these later.', 'Pick the block length, then fill each day.'];
	const short = (date: string) => formatDate(date, { day: 'numeric', month: 'short' });

	async function save() {
		busy = true;
		error = '';
		try {
			const body = $state.snapshot(draft);
			body.days = body.days.filter((d) => d.day <= body.block_days);
			const saved = await saveCycle(body);
			if (!saved.leftOut) return await goto(`/plan/cycles/${draft.id}`, { replaceState: true });
			leftOut = saved.leftOut;
		} catch (e) {
			error = describe(e, (f) => (f.startsWith('days') ? 'A session' : f.startsWith('goals') ? 'A goal' : (fields[f] ?? f)));
		} finally {
			busy = false;
		}
	}

	function next(e: SubmitEvent) {
		e.preventDefault();
		if (step < 3) goto(`?step=${step + 1}`, { keepFocus: false, noScroll: false });
		else save();
	}
</script>

<svelte:head><title>New cycle</title></svelte:head>

<NavBar
	title="New cycle"
	heading={false}
	back={step === 1 ? { href: '/plan?view=cycles', label: 'Cancel' } : { href: `?step=${step - 1}`, label: 'Back' }}
>
	{#snippet actions()}
		<span class="pr-1 text-xs font-bold text-ink-2">{step} of 3</span>
	{/snippet}
</NavBar>

<form class="flex flex-col gap-3.5 px-4 pt-1 pb-[calc(env(safe-area-inset-bottom)+5rem)]" onsubmit={next}>
	<ol class="grid grid-cols-3 gap-1.5" aria-hidden="true">
		{#each [1, 2, 3] as s (s)}
			<li class="h-1 rounded-full {s <= step ? 'bg-ink' : 'bg-well'}"></li>
		{/each}
	</ol>

	{#if step > 1}
		<p class="flex flex-wrap gap-2 text-xs font-bold">
			<span class="rounded-full bg-surface px-3 py-1.5 shadow-card-sm">{draft.name}</span>
			<span class="rounded-full bg-surface px-3 py-1.5 shadow-card-sm">{short(draft.starts)} – {short(draft.ends)}</span>
		</p>
	{/if}

	<header class="px-1">
		<h1 class="text-[32px] leading-[1.1] font-extrabold tracking-[-0.02em]">{titles[step - 1]}</h1>
		<p class="mt-1 text-[15px] font-semibold text-ink-2">{hints[step - 1]}</p>
	</header>

	{#if step === 1}
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
		</div>
		<div class="grid grid-cols-3 gap-2">
			{#each [4, 6, 8] as weeks (weeks)}
				<button
					type="button"
					class="h-12 rounded-full text-[15px] font-bold {length === weeks * 7 ? 'bg-ink text-ground' : 'bg-surface text-ink shadow-card-sm'}"
					aria-pressed={length === weeks * 7}
					onclick={() => (draft.ends = addDays(draft.starts, weeks * 7 - 1))}
				>
					{weeks} weeks
				</button>
			{/each}
		</div>
	{:else if step === 2}
		<section class="flex flex-col gap-2">
			<h2 class="px-1 text-xs font-semibold text-ink-2">Goals</h2>
			<GoalList bind:goals={draft.goals} />
		</section>
		<section class="flex flex-col gap-2">
			<h2 class="px-1 text-xs font-semibold text-ink-2">Before · where you start</h2>
			<div class="rounded-3xl bg-surface px-[18px] pb-1 shadow-card">
				<EntryList bind:entries={draft.before} label="New before entry" />
			</div>
		</section>
		<section class="flex flex-col gap-2">
			<h2 class="px-1 text-xs font-semibold text-ink-2">Notes</h2>
			<textarea
				class="min-h-28 w-full resize-none rounded-3xl bg-surface px-[18px] py-3.5 text-[15px] font-semibold shadow-card [field-sizing:content] placeholder:text-ink-3 focus:outline-none"
				placeholder="Anything to remember about this cycle"
				aria-label="Notes"
				value={draft.notes ?? ''}
				oninput={(e) => (draft.notes = e.currentTarget.value || null)}
			></textarea>
		</section>
	{:else}
		<div class="grid grid-cols-4 gap-2">
			{#each [7, 10, 14] as n (n)}
				<button
					type="button"
					class="h-12 rounded-full text-[15px] font-bold {draft.block_days === n ? 'bg-ink text-ground' : 'bg-surface text-ink shadow-card-sm'}"
					aria-pressed={draft.block_days === n}
					disabled={n > length}
					onclick={() => (draft.block_days = n)}
				>
					{n} days
				</button>
			{/each}
			<input
				class="input h-12 px-2 text-center"
				type="number"
				inputmode="numeric"
				min="1"
				max={Math.min(28, length)}
				aria-label="Days in the block"
				bind:value={draft.block_days}
				required
			/>
		</div>
		<section class="rounded-3xl bg-surface px-[18px] pt-1 pb-2 shadow-card">
			<BlockDays bind:days={draft.days} blockDays={draft.block_days} from={draft.starts} templates={data.templates} />
		</section>
	{/if}

	<FormError message={error} />
	{#if leftOut}
		<p class="rounded-3xl bg-surface p-[18px] text-[15px] font-semibold shadow-card">
			Saved. {leftOut === 1 ? '1 day' : `${leftOut} days`} already held that session, so the cycle left
			{leftOut === 1 ? 'it as it was' : 'them as they were'}.
			<a href="/plan/cycles/{draft.id}" class="font-bold text-link underline">Open the cycle</a>
		</p>
	{:else}
		<div class="flex items-center gap-2">
			{#if step === 2}
				<div class="w-28 shrink-0"><Button variant="secondary" href="?step=3">Skip</Button></div>
			{/if}
			<Button type="submit" disabled={busy}>{step < 3 ? 'Next' : 'Save cycle'}</Button>
		</div>
	{/if}
</form>
