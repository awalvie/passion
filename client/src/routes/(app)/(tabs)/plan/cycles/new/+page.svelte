<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { untrack } from 'svelte';
	import { describe } from '$lib/api';
	import BlockDays from '$lib/BlockDays.svelte';
	import BlockLength from '$lib/BlockLength.svelte';
	import Button from '$lib/Button.svelte';
	import CyclePreview from '$lib/CyclePreview.svelte';
	import { addDays, daysBetween, formatDate } from '$lib/dates';
	import DateField from '$lib/DateField.svelte';
	import FormError from '$lib/FormError.svelte';
	import GoalList from '$lib/GoalList.svelte';
	import GoalSheet from '$lib/GoalSheet.svelte';
	import Icon, { type IconName } from '$lib/Icon.svelte';
	import NavBar from '$lib/NavBar.svelte';
	import { plannedDays, saveCycle } from '$lib/plan';
	import SessionIcon from '$lib/SessionIcon.svelte';

	let { data } = $props();

	// The step lives in the address, so the phone's back gesture steps back.
	// The draft keeps the id it opened with, so a retry makes one cycle.
	const step = $derived(Math.min(4, Math.max(1, Number(page.url.searchParams.get('step')) || 1)));
	let draft = $state(untrack(() => structuredClone($state.snapshot(data.cycle))));
	let busy = $state(false);
	let error = $state('');
	let goalOpen = $state(false);
	let goalIndex = $state(-1);

	const length = $derived(daysBetween(draft.starts, draft.ends) + 1);
	const fields: Record<string, string> = { name: 'The name', starts: 'The start', ends: 'The end', block_days: 'The block' };
	const weeks = $derived(Math.ceil(length / 7));
	const planned = $derived(plannedDays(draft, data.today));
	const titles = ['Name and dates', 'What is it for?', 'What repeats?', 'Check it'];
	const hints = $derived([
		'The block comes next.',
		'All optional. You can add these later.',
		'Pick the block length, then fill each day.',
		`${planned.length === 1 ? '1 session' : `${planned.length} sessions`} over ${weeks === 1 ? '1 week' : `${weeks} weeks`}.`
	]);
	const short = (date: string) => formatDate(date, { day: 'numeric', month: 'short' });
	const byId = $derived(new Map(data.templates.map((t) => [t.id, t])));
	const blockList = $derived(
		Array.from({ length: draft.block_days }, (_, i) => draft.days.find((d) => d.day === i + 1)).map((d) => d && byId.get(d.template))
	);
	const count = (n: number, one: string) => `${n} ${one}${n === 1 ? '' : 's'}`;
	const goalsLine = $derived(
		[draft.goals.length ? count(draft.goals.length, 'goal') : '', draft.notes ? 'notes' : ''].filter(Boolean).join(' · ') || 'None yet'
	);

	async function save() {
		busy = true;
		error = '';
		try {
			const body = $state.snapshot(draft);
			body.days = body.days.filter((d) => d.day <= body.block_days);
			const { leftOut } = await saveCycle(body);
			await goto(`/plan/cycles/${draft.id}`, { replaceState: true, state: { leftOut } });
		} catch (e) {
			error = describe(e, (f) => (f.startsWith('days') ? 'A session' : f.startsWith('goals') ? 'A goal' : (fields[f] ?? f)));
		} finally {
			busy = false;
		}
	}

	function next(e: SubmitEvent) {
		e.preventDefault();
		if (step < 4) goto(`?step=${step + 1}`, { keepFocus: false, noScroll: false });
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
		<span class="pr-1 text-xs font-bold text-ink-2">{step} of 4</span>
	{/snippet}
</NavBar>

<form class="flex min-h-[calc(100dvh-4rem)] flex-col gap-3.5 px-4 pt-1" onsubmit={next}>
	<ol class="grid grid-cols-4 gap-1.5" aria-hidden="true">
		{#each [1, 2, 3, 4] as s (s)}
			<li class="h-1 rounded-full {s <= step ? 'bg-ink' : 'bg-well'}"></li>
		{/each}
	</ol>

	{#if step === 2 || step === 3}
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
					<DateField bind:value={draft.starts} label="Starts" />
				</label>
				<label class="flex flex-col gap-1.5 text-xs font-semibold text-ink-2">
					Ends
					<DateField bind:value={draft.ends} label="Ends" min={draft.starts} />
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
			<GoalList
				bind:goals={draft.goals}
				onedit={(i) => {
					goalIndex = i;
					goalOpen = true;
				}}
			/>
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
	{:else if step === 3}
		<BlockLength bind:value={draft.block_days} max={length} />
		<BlockDays bind:days={draft.days} blockDays={draft.block_days} from={draft.starts} templates={data.templates} />
	{:else}
		<ul class="rounded-3xl bg-surface px-3.5 shadow-card">
			{#snippet row(icon: IconName, eyebrow: string, to: number, body: import('svelte').Snippet)}
				<li class="flex min-h-[68px] items-center gap-3 py-2.5 [&:not(:first-child)]:shadow-[inset_0_1px_0_var(--line)]">
					<span class="flex size-10 shrink-0 items-center justify-center rounded-xl bg-well text-ink"><Icon name={icon} size="1.125rem" /></span>
					<span class="min-w-0 flex-1">
						<span class="block text-xs font-semibold text-ink-2">{eyebrow}</span>
						{@render body()}
					</span>
					<a href="?step={to}" class="flex min-h-11 shrink-0 items-center px-1 text-xs font-bold text-ink-2">Change</a>
				</li>
			{/snippet}
			{#snippet name()}<span class="block truncate text-[15px] font-bold">{draft.name}</span>{/snippet}
			{#snippet block()}
				<span class="mt-1 flex flex-wrap items-center gap-1">
					{#each blockList as t, i (i)}
						{#if t}
							<SessionIcon icon={t.icon} name={t.name} size={22} />
						{:else}
							<span class="flex h-[22px] w-3 items-center justify-center"><span class="size-1 rounded-full bg-ink-3"></span></span>
						{/if}
					{/each}
				</span>
			{/snippet}
			{#snippet goals()}<span class="block truncate text-[15px] font-bold">{goalsLine}</span>{/snippet}
			{@render row('pencil', `${short(draft.starts)} – ${short(draft.ends)} · ${count(weeks, 'week')}`, 1, name)}
			{@render row('repeat', `Block · every ${draft.block_days} days`, 3, block)}
			{@render row('target', 'Goals and notes', 2, goals)}
		</ul>
		<CyclePreview starts={draft.starts} ends={draft.ends} {planned} templates={data.templates} />
	{/if}

	<FormError message={error} />
	<div class="sticky bottom-0 -mx-4 mt-auto flex items-center gap-2 bg-ground px-4 pt-3 pb-[calc(env(safe-area-inset-bottom)+1rem)]">
		{#if step === 2}
			<div class="w-28 shrink-0"><Button variant="secondary" href="?step=3">Skip</Button></div>
		{/if}
		<Button type="submit" disabled={busy}>{step < 3 ? 'Next' : step === 3 ? 'Next: see it on the calendar' : busy ? 'Creating…' : 'Create cycle'}</Button>
	</div>
</form>

<GoalSheet bind:open={goalOpen} goals={draft.goals} index={goalIndex} onsave={(next) => (draft.goals = next)} />
