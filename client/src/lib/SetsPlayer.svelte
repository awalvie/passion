<script lang="ts">
	import { request } from './api';
	import { formatDate } from './dates';
	import Icon from './Icon.svelte';
	import { isFinished, setsOf, type HistorySession, type RunStep, type SetFields } from './run';
	import { openRun } from './runState.svelte';
	import RunPage, { type PlayerProps } from './RunPage.svelte';
	import Sheet from './Sheet.svelte';

	let { step, next, nextHref, last, menu, error, leaving, leave, skip }: PlayerProps = $props();

	// A per-side set logs two rows, left then right. The server has no side
	// column, so the order carries it. The logged rows are the sets marked
	// done, always from the first; the rest are drafts on this phone.
	const sides = $derived(step.per_side ? 2 : 1);
	const logged = $derived(setsOf(openRun.run!, step.id));
	const count = $derived(Math.max(step.sets ?? 0, Math.ceil(logged.length / sides), 1));
	const doneSets = $derived(Math.floor(logged.length / sides));

	let past = $state<HistorySession[]>([]);
	// The page stays mounted from one exercise to the next, so the history of
	// the one before can still be loaded.
	const sessions = $derived(past[0]?.sets[0]?.exercise === step.exercise ? past : []);
	const history = $derived(sessions[0] ?? null);
	let browsing = $state(false);

	$effect(() => {
		const exercise = step.exercise;
		let current = true;
		past = [];
		request<{ sessions: HistorySession[] }>('GET', `/api/v1/exercises/${exercise}/history`)
			.then((h) => {
				if (current) past = h.sessions.filter((s) => s.sets.length);
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

	// A set the timer logged keeps its times.
	const rows = (upTo: number) =>
		Array.from({ length: upTo }, (_, k) => {
			const s = value(k);
			return { reps: s.reps, seconds: s.seconds, rep_seconds: s.rep_seconds, weight_kg: s.weight_kg };
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
		// Fewer reps on a ladder set drop the times of the reps taken away.
		if (field === 'reps' && s.rep_seconds && n !== null) s.rep_seconds = s.rep_seconds.slice(0, n);
		if (k < logged.length) openRun.setSets(step, logged.map((l, j) => (j === k ? s : l)));
		else drafts[k] = s;
	}

	// Drafts after the removed set move up one set.
	function remove(i: number) {
		const moved: Record<number, SetFields> = {};
		for (const [k, v] of Object.entries(drafts)) {
			const at = Math.floor(Number(k) / sides);
			if (at !== i) moved[at > i ? Number(k) - sides : Number(k)] = v;
		}
		drafts = moved;
		openRun.removeSet(step, i, count);
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
	const day = (date: string) => formatDate(date, { weekday: 'short', day: 'numeric', month: 'short' });
	const short = (s: SetFields) => [s.reps !== null ? `${s.reps}` : '', s.weight_kg !== null ? `${s.weight_kg} kg` : ''].filter(Boolean).join(' × ') || '–';
</script>

{#snippet field(k: number, key: 'reps' | 'weight_kg', state: 'done' | 'now' | 'later', label: string)}
	{@const v = value(k)[key]}
	{@const then = history?.sets[k]?.[key] ?? null}
	<span class="relative min-w-0">
		<input
			class="h-12 w-full rounded-xl text-center font-[family-name:var(--font-digits)] text-2xl leading-none font-bold outline-none placeholder:font-sans placeholder:text-base placeholder:font-semibold placeholder:text-(--fg2)/55 focus:ring-2 focus:ring-tint
				{then !== null ? 'pb-3' : ''}
				{state === 'done' ? 'bg-black/7' : state === 'now' ? 'bg-black/30 ring-1 ring-white/6' : 'bg-black/16 text-(--fg2)'}"
			inputmode={key === 'reps' ? 'numeric' : 'decimal'}
			placeholder={key === 'reps' ? 'reps' : 'kg'}
			aria-label="{label} {key === 'reps' ? 'reps' : 'weight, kg'}"
			value={v ?? ''}
			onfocus={select}
			onchange={(e) => edit(k, key, e.currentTarget.value)}
		/>
		{#if then !== null}
			<span class="pointer-events-none absolute inset-x-0 bottom-1.5 text-center text-[11px] leading-none font-semibold text-(--fg2)">last {then}</span>
		{/if}
	</span>
{/snippet}

<RunPage {step} {next} {menu} {error} done={finished ? { href: nextHref, last } : null} {skip}>
	<div class="flex min-h-0 flex-1 flex-col gap-2.5 overflow-y-auto overscroll-contain px-4 pt-6 pb-4">
		<ol class="overflow-hidden rounded-[18px] bg-(--glass)">
			{#if history}
				<li>
					<button
						type="button"
						class="flex h-12 w-full items-center gap-2.5 px-3.5 text-left text-[15px] font-semibold text-(--fg2) shadow-[inset_0_-1px_0_rgba(255,255,255,0.06)]"
						onclick={() => (browsing = true)}
					>
						<span class="flex size-7 shrink-0 items-center justify-center rounded-full bg-(--glass) text-(--fg)"><Icon name="history" size="15px" stroke={2.4} /></span>
						<span class="min-w-0 flex-1 truncate">Last time · <b class="text-(--fg)">{day(history.local_date)}</b></span>
						<span class="flex items-center gap-1">History<Icon name="chevron-right" size="16px" stroke={2.6} /></span>
					</button>
				</li>
			{/if}
			<li class="grid gap-x-2.5 px-3.5 pt-3 pb-1.5 text-xs font-bold tracking-[0.1em] text-(--fg2)/80 {sides === 2 ? 'grid-cols-[40px_48px_1fr_1fr_28px]' : 'grid-cols-[40px_1fr_1fr_28px]'}">
				<span class="col-start-[-4] text-center">REPS</span><span class="text-center">WEIGHT</span>
			</li>
			{#each { length: count }, i (i)}
				{@const state = i < doneSets ? 'done' : i === current ? 'now' : 'later'}
				<li
					class="relative grid items-center gap-x-2.5 gap-y-2 px-3.5 py-3 {i ? 'shadow-[inset_0_1px_0_rgba(255,255,255,0.06)]' : ''}
						{sides === 2 ? 'grid-cols-[40px_48px_1fr_1fr_28px]' : 'grid-cols-[40px_1fr_1fr_28px]'}
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
						{#if s === 0}
							<button
								type="button"
								class="relative flex size-7 items-center justify-center rounded-full bg-(--glass) text-(--fg2) before:absolute before:-inset-2"
								style="grid-row: span {sides}"
								aria-label="Remove set {i + 1}"
								onclick={() => remove(i)}
							>
								<Icon name="x" size="12px" stroke={3} />
							</button>
						{/if}
					{/each}
				</li>
			{/each}
			<li class="shadow-[inset_0_1px_0_rgba(255,255,255,0.06)]">
				<button type="button" class="flex h-[52px] w-full items-center justify-center gap-2 text-[15px] font-bold" onclick={() => openRun.addSet(step, count)}>
					<span class="flex size-7 items-center justify-center rounded-full bg-(--glass)"><Icon name="plus" size="16px" stroke={2.6} /></span>Add set
				</button>
			</li>
		</ol>
	</div>

	<Sheet bind:open={browsing} title="History" eyebrow={step.name}>
		<div class="flex flex-col gap-3 px-1">
			{#each sessions.slice(0, 5) as session (session.run)}
				<section class="rounded-[22px] bg-surface p-4 shadow-card-sm">
					<p class="text-xs font-semibold text-ink-2">{day(session.local_date)}</p>
					<ol class="mt-2 flex flex-col gap-1.5">
						{#each { length: Math.ceil(session.sets.length / sides) }, i (i)}
							<li class="flex items-baseline gap-3 text-[15px] font-semibold">
								<span class="w-12 shrink-0 text-ink-2">Set {i + 1}</span>
								<span class="font-[family-name:var(--font-digits)] text-lg font-bold">
									{session.sets
										.slice(i * sides, (i + 1) * sides)
										.map(short)
										.join(' / ')}
								</span>
							</li>
						{/each}
					</ol>
				</section>
			{/each}
		</div>
	</Sheet>

	{#snippet buttons()}
		<button type="button" class="run-btn bg-tint text-on-tint" onclick={done}>
			<span class="run-icon"><Icon name="check" size="16px" stroke={3} /></span>Done
		</button>
	{/snippet}
</RunPage>
