<script lang="ts">
	import { request } from './api';
	import Icon from './Icon.svelte';
	import type { MenuItem } from './Menu.svelte';
	import { isFinished, setsOf, type HistorySession, type RunStep, type SetFields } from './run';
	import { openRun } from './runState.svelte';
	import RunPage from './RunPage.svelte';
	import type { NextLine } from './template';

	let {
		step,
		next,
		nextHref,
		last,
		menu,
		error,
		leaving,
		leave,
		skip
	}: {
		step: RunStep;
		next: NextLine;
		nextHref: string;
		last: boolean;
		menu: MenuItem[];
		error: string;
		leaving: boolean;
		leave: (end: () => void) => Promise<void>;
		skip: () => void;
	} = $props();

	// A per-side set logs two rows, left then right. The server has no side
	// column, so the order carries it. The logged rows are the sets marked
	// done, always from the first; the rest are drafts on this phone.
	const sides = $derived(step.per_side ? 2 : 1);
	const logged = $derived(setsOf(openRun.run!, step.id));
	const count = $derived(Math.max(step.sets ?? 0, Math.ceil(logged.length / sides), 1));
	const doneSets = $derived(Math.floor(logged.length / sides));

	let lastTime = $state<HistorySession | null>(null);
	// The page stays mounted from one exercise to the next, so the history of
	// the one before can still be loaded.
	const history = $derived(lastTime?.sets[0]?.exercise === step.exercise ? lastTime : null);

	$effect(() => {
		const exercise = step.exercise;
		let current = true;
		lastTime = null;
		request<{ sessions: HistorySession[] }>('GET', `/api/v1/exercises/${exercise}/history`)
			.then((h) => {
				if (current) lastTime = h.sessions.find((s) => s.sets.length) ?? null;
			})
			.catch(() => {});
		return () => (current = false);
	});

	// Edits to sets not done yet, by row.
	let drafts = $state<Record<number, SetFields>>({});
	$effect.pre(() => {
		void step.id;
		drafts = {};
	});

	// A draft starts from the same set last time, else the set before it, else
	// the plan.
	function prefill(k: number): SetFields {
		const then = history?.sets[k];
		if (then) return { reps: then.reps, seconds: null, weight_kg: then.weight_kg };
		if (k >= sides) return value(k - sides);
		return { reps: step.reps, seconds: null, weight_kg: null };
	}

	function value(k: number): SetFields {
		return k < logged.length ? logged[k] : (drafts[k] ?? prefill(k));
	}

	const rows = (upTo: number) =>
		Array.from({ length: upTo }, (_, k) => {
			const s = value(k);
			return { reps: s.reps, seconds: null, weight_kg: s.weight_kg };
		});

	// A tap on an open set logs it and every set before it; on a done set it
	// opens that set and every set after it again.
	function toggle(i: number) {
		if (i < doneSets) {
			for (let k = i * sides; k < logged.length; k++) drafts[k] = { ...logged[k] };
			openRun.unlog(step, rows(i * sides));
			return;
		}
		const upTo = (i + 1) * sides;
		openRun.setSets(step, rows(upTo));
		for (let k = 0; k < upTo; k++) delete drafts[k];
		if (i + 1 >= count) openRun.finish(step);
	}

	function edit(k: number, field: 'reps' | 'weight_kg', text: string) {
		const n = text.trim() === '' ? null : Number(text.replace(',', '.'));
		if (n !== null && !Number.isFinite(n)) return;
		const s = { ...value(k), [field]: n };
		if (k < logged.length) openRun.setSets(step, logged.map((l, j) => (j === k ? s : l)));
		else drafts[k] = s;
	}

	function done() {
		void leave(() => {
			openRun.setSets(step, rows(count * sides));
			openRun.finish(step);
		});
	}

	const finished = $derived(isFinished(step) && !leaving);
	const current = $derived(finished ? -1 : doneSets);
	const select = (e: FocusEvent) => (e.currentTarget as HTMLInputElement).select();
</script>

{#snippet field(k: number, key: 'reps' | 'weight_kg', state: 'done' | 'now' | 'later', label: string)}
	{@const v = value(k)[key]}
	<input
		class="h-12 w-full min-w-0 rounded-xl text-center font-[family-name:var(--font-digits)] text-2xl font-bold outline-none placeholder:font-sans placeholder:text-base placeholder:font-semibold placeholder:text-(--fg2)/55 focus:ring-2 focus:ring-tint
			{state === 'done' ? 'bg-black/7' : state === 'now' ? 'bg-black/30 ring-1 ring-white/6' : 'bg-black/16 text-(--fg2)'}"
		inputmode={key === 'reps' ? 'numeric' : 'decimal'}
		placeholder={key === 'reps' ? 'reps' : 'kg'}
		aria-label="{label} {key === 'reps' ? 'reps' : 'weight, kg'}"
		value={v ?? ''}
		onfocus={select}
		onchange={(e) => edit(k, key, e.currentTarget.value)}
	/>
{/snippet}

<RunPage {step} {next} {menu} {error}>
	<div class="flex min-h-0 flex-1 flex-col gap-2.5 overflow-y-auto overscroll-contain px-4 pt-6 pb-4">
		<p class="grid gap-x-2.5 px-3.5 text-xs font-bold tracking-[0.1em] text-(--fg2)/80 {sides === 2 ? 'grid-cols-[40px_48px_1fr_1fr]' : 'grid-cols-[40px_1fr_1fr]'}">
			<span class="col-start-[-3] text-center">REPS</span><span class="text-center">WEIGHT</span>
		</p>
		<ol class="overflow-hidden rounded-[18px] bg-(--glass)">
			{#each { length: count }, i (i)}
				{@const state = i < doneSets ? 'done' : i === current ? 'now' : 'later'}
				<li
					class="relative grid items-center gap-x-2.5 gap-y-2 px-3.5 py-3 [&:not(:first-child)]:shadow-[inset_0_1px_0_rgba(255,255,255,0.06)]
						{sides === 2 ? 'grid-cols-[40px_48px_1fr_1fr]' : 'grid-cols-[40px_1fr_1fr]'}
						{state === 'done' ? 'bg-rest-on-hero/14' : state === 'now' ? 'bg-white/7' : ''}"
				>
					{#if state === 'now'}<i class="absolute inset-y-2.5 left-0 w-[3px] rounded-r-[3px] bg-tint"></i>{/if}
					<button
						type="button"
						class="relative flex size-10 items-center justify-center rounded-full font-[family-name:var(--font-digits)] text-lg font-bold before:absolute before:-inset-1
							{state === 'done'
							? 'bg-rest-on-hero text-on-tint'
							: state === 'now'
								? 'text-tint shadow-[inset_0_0_0_2px_var(--tint)]'
								: 'text-(--fg2) shadow-[inset_0_0_0_1.5px_color-mix(in_srgb,var(--fg2)_40%,transparent)]'}"
						style="grid-row: span {sides}"
						aria-label="Set {i + 1} done"
						aria-pressed={state === 'done'}
						onclick={() => toggle(i)}
					>
						{#if state === 'done'}<Icon name="check" size="18px" stroke={3} />{:else}{i + 1}{/if}
					</button>
					{#each { length: sides }, s (s)}
						{#if sides === 2}<span class="text-sm font-semibold text-(--fg2) {state === 'later' ? 'opacity-70' : ''}">{s ? 'Right' : 'Left'}</span>{/if}
						{@const label = `Set ${i + 1}${sides === 2 ? (s ? ' right' : ' left') : ''}`}
						{@render field(i * sides + s, 'reps', state, label)}
						{@render field(i * sides + s, 'weight_kg', state, label)}
					{/each}
				</li>
			{/each}
		</ol>
	</div>

	{#snippet buttons()}
		{#if finished}
			<a href={nextHref} class="run-btn bg-live text-on-live">
				{last ? 'Back to session' : 'Next exercise'}<span class="run-icon"><Icon name="arrow-right" size="16px" stroke={2.6} /></span>
			</a>
		{:else}
			<button type="button" class="run-btn bg-tint text-on-tint" onclick={done}>
				<span class="run-icon"><Icon name="check" size="16px" stroke={3} /></span>Done
			</button>
			<button type="button" class="run-round" aria-label="Skip" onclick={skip}><Icon name="skip" size="26px" /></button>
		{/if}
	{/snippet}
</RunPage>
