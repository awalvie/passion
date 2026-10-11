<script lang="ts">
	import { untrack } from 'svelte';
	import { loadGrades, scaleFor, type Grades } from './grades';
	import Icon from './Icon.svelte';
	import { newId } from './id';
	import type { MenuItem } from './Menu.svelte';
	import type { Account } from './plan';
	import { climbsOf, isFinished, type Climb, type ClimbFields, type Discipline, type Go, type RunStep } from './run';
	import { openRun } from './runState.svelte';
	import RunPage from './RunPage.svelte';
	import Segmented from './Segmented.svelte';
	import Sheet from './Sheet.svelte';
	import Toast from './Toast.svelte';
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

	const logged = $derived(climbsOf(openRun.run!, step.id));

	let info = $state<{ grades: Grades; account: Account } | null>(null);
	$effect(() => {
		loadGrades()
			.then((g) => (info = g))
			.catch(() => {});
	});

	const disciplines: Record<Discipline, string> = { boulder: 'Boulder', sport: 'Sport', trad: 'Trad' };
	const settings = { indoor: 'Indoor', outdoor: 'Outdoor' } as const;
	const ropes = { lead: 'Lead', top_rope: 'Top rope', auto_belay: 'Auto belay', follow: 'Follow' } as const;

	// The climb on the card. It reaches the server at its first go, and stays
	// the current one until "Add climb", so after a reload the last climb of
	// an unfinished step is current again.
	type Current = { id: string; grade: string | null; goes: Go[]; focus: string | null; notes: string | null };
	let cur = $state<Current>({ id: '', grade: null, goes: [], focus: null, notes: null });
	// The bar sets every new climb, and starts from the last one logged.
	let discipline = $state<Discipline>('boulder');
	let setting = $state<keyof typeof settings>('indoor');
	let rope = $state<keyof typeof ropes>('lead');
	const fresh = (grade: string | null): Current => ({ id: newId(), grade, goes: [], focus: null, notes: null });
	const resume = (c: Climb): Current => ({ id: c.id, grade: c.grade, goes: [...c.goes], focus: c.focus, notes: c.notes });
	$effect.pre(() => {
		const id = step.id;
		untrack(() => {
			const end = climbsOf(openRun.run!, id).at(-1);
			cur = end && !isFinished(step) ? resume(end) : fresh(end?.grade ?? null);
			discipline = end?.discipline ?? 'boulder';
			setting = end?.setting ?? 'indoor';
			rope = (end?.rope_style as keyof typeof ropes | null) ?? 'lead';
		});
	});
	const gradesFor = (d: Discipline) => {
		const sc = info ? scaleFor(info.grades, info.account, d === 'boulder') : undefined;
		return [...(sc?.grades ?? []), ...(info?.grades.ungraded ?? [])];
	};
	const gradeList = $derived(gradesFor(discipline));

	// The scale a grade belongs to: none for an ungraded label or no grade.
	// Before the scales load, as offline, a grade kept from an earlier climb
	// keeps that climb's scale.
	function systemFor(d: Discipline, grade: string | null) {
		if (!grade) return null;
		if (!info) return logged.findLast((l) => l.grade === grade)?.grade_system ?? null;
		if (info.grades.ungraded.includes(grade)) return null;
		return scaleFor(info.grades, info.account, d === 'boulder')?.system ?? null;
	}

	const ended = $derived(logged.filter((c) => c.id !== cur.id));
	const sends = $derived(ended.filter((c) => c.sent).length);

	// An ungraded climb is never a send (rule 39), so it counts its goes.
	function word(c: Climb) {
		const n = c.goes.filter((g) => g === 'send').length;
		if (!c.grade_system) return `${c.goes.length} ${c.goes.length === 1 ? 'go' : 'goes'}`;
		return n > 1 ? `${n} sends` : n ? (c.goes.length === 1 ? 'Flash' : 'Sent') : 'Not sent';
	}

	function save(c: Current) {
		const had = logged.find((l) => l.id === c.id);
		openRun.putClimb(step, c.id, {
			step: step.id,
			position: had?.position ?? Math.max(-1, ...logged.map((l) => l.position)) + 1,
			discipline,
			setting,
			board: discipline === 'boulder' ? (had?.board ?? null) : null,
			rope_style: discipline === 'boulder' ? null : rope,
			grade: c.grade,
			grade_system: systemFor(discipline, c.grade),
			goes: [...c.goes],
			seconds: had?.seconds ?? null,
			stars: had?.stars ?? null,
			focus: c.focus,
			notes: c.notes
		});
	}

	// A logged climb is written again whole, with what changed.
	function rewrite(c: Climb, changes: Partial<ClimbFields>) {
		const { id, exercise: _e, sent: _s, ...fields } = c;
		openRun.putClimb(step, id, { ...fields, ...changes });
	}

	let editing = $state<string | null>(null);
	const editingClimb = $derived(editing === null || editing === cur.id ? null : logged.find((c) => c.id === editing));
	const editingNumber = $derived(editing === cur.id ? ended.length + 1 : ended.findIndex((c) => c.id === editing) + 1);

	function setNote(key: 'focus' | 'notes', text: string) {
		const value = text.trim() || null;
		if (editingClimb) return rewrite(editingClimb, { [key]: value });
		cur[key] = value;
		if (cur.goes.length) save(cur);
	}

	// A count changed in the sheet keeps the order of the goes it leaves, and
	// a climb keeps at least one go.
	function setCount(kind: Go, n: number) {
		const c = editingClimb ?? cur;
		const goes = [...c.goes];
		const have = goes.filter((g) => g === kind).length;
		if (n < 0 || (n === 0 && goes.length === have)) return;
		if (n > have) goes.push(kind);
		else goes.splice(goes.lastIndexOf(kind), 1);
		if (editingClimb) return rewrite(editingClimb, { goes });
		cur.goes = goes;
		save(cur);
	}

	function setGrade(c: Climb, g: string) {
		const grade = c.grade === g ? null : g;
		rewrite(c, { grade, grade_system: systemFor(c.discipline, grade) });
	}

	function remove(c: Climb) {
		editing = null;
		openRun.removeClimb(step, c.id);
	}

	function go(kind: Go) {
		toast = '';
		cur.goes.push(kind);
		save(cur);
	}

	// A second tap on the picked grade clears it: a climb may have no grade.
	function pick(g: string) {
		cur.grade = cur.grade === g ? null : g;
		if (cur.goes.length) save(cur);
	}

	// A boulder and a route use different scales, so a change between them
	// clears the grade.
	let changing = $state(false);
	function setDiscipline(d: Discipline) {
		if ((d === 'boulder') !== (discipline === 'boulder')) cur.grade = null;
		discipline = d;
		if (cur.goes.length) save(cur);
	}
	function setSetting(v: keyof typeof settings) {
		setting = v;
		if (cur.goes.length) save(cur);
	}
	function setRope(v: keyof typeof ropes) {
		rope = v;
		if (cur.goes.length) save(cur);
	}

	// Undo brings the climb just ended back onto the card, until the new one
	// has a go.
	let toast = $state('');
	let undoTo = '';
	function addClimb() {
		const c = logged.find((l) => l.id === cur.id)!;
		toast = [`Climb ${ended.length + 1}`, c.grade, word(c)].filter(Boolean).join(' · ');
		undoTo = cur.id;
		cur = fresh(cur.grade);
	}
	function undo() {
		const c = logged.find((l) => l.id === undoTo);
		if (c) cur = resume(c);
	}

	function done() {
		void leave(() => openRun.finish(step));
	}

	// Keeps the picked grade, or the middle of the scale, in the middle of the
	// strip.
	function center(node: HTMLElement, _key: unknown) {
		const scroll = () => {
			const el = node.querySelector<HTMLElement>('[aria-pressed="true"]');
			const left = el ? el.offsetLeft + el.offsetWidth / 2 : node.scrollWidth / 2;
			node.scrollTo({ left: left - node.clientWidth / 2, behavior: 'smooth' });
		};
		requestAnimationFrame(scroll);
		return { update: () => requestAnimationFrame(scroll) };
	}

	const finished = $derived(isFinished(step) && !leaving);
	const system = $derived(info ? (scaleFor(info.grades, info.account, discipline === 'boulder')?.system.toUpperCase() ?? '') : '');
</script>

{#snippet goDot(g: Go, n: number)}
	<span
		class="flex size-[30px] shrink-0 items-center justify-center rounded-full {g === 'send'
			? 'bg-rest-on-hero text-on-tint'
			: 'bg-white/14 font-[family-name:var(--font-digits)] text-[15px] font-bold'}"
	>
		{#if g === 'send'}<Icon name="check" size="15px" stroke={3} /><span class="sr-only">Send</span>{:else}<span class="sr-only">Attempt</span>{n}{/if}
	</span>
{/snippet}

{#snippet strip(selected: string | null, list: string[], onpick: (g: string) => void, big = false)}
	<div
		class="relative -mx-1 flex items-center gap-1.5 overflow-x-auto px-1 py-0.5 [scrollbar-width:none] [mask-image:linear-gradient(90deg,transparent,#000_24px,#000_calc(100%-24px),transparent)] {big
			? 'h-[76px]'
			: ''}"
		use:center={[selected, list.length]}
		role="group"
		aria-label="Grade"
	>
		{#each list as g (g)}
			<button
				type="button"
				class="shrink-0 rounded-xl px-2.5 font-[family-name:var(--font-digits)] font-bold {big
					? selected === g
						? 'h-[72px] min-w-[92px] rounded-2xl text-[46px]'
						: 'h-14 min-w-14 text-2xl'
					: 'h-11 min-w-12 text-xl'} {selected === g ? 'bg-(--fg) text-on-tint' : 'bg-black/20 text-(--fg2)'}"
				aria-pressed={selected === g}
				onclick={() => onpick(g)}>{g}</button
			>
		{/each}
	</div>
{/snippet}

{#snippet count(kind: Go, label: string, n: number)}
	<div class="flex items-center gap-2 rounded-[18px] bg-surface p-2 pl-3">
		<span class="flex size-7 items-center justify-center rounded-full {kind === 'send' ? 'bg-rest-on-hero text-on-tint' : 'bg-white/14'}"
			><Icon name={kind === 'send' ? 'check' : 'plus'} size="14px" stroke={3} /></span
		>
		<span class="min-w-0 flex-1 truncate text-[15px] font-bold">{label}</span>
		<button type="button" class="flex size-11 items-center justify-center rounded-full bg-well" aria-label="One {label.toLowerCase().slice(0, -1)} less" onclick={() => setCount(kind, n - 1)}
			><Icon name="minus" size="18px" stroke={2.6} /></button
		>
		<span class="w-8 text-center font-[family-name:var(--font-digits)] text-2xl font-bold">{n}</span>
		<button type="button" class="flex size-11 items-center justify-center rounded-full bg-well" aria-label="One {label.toLowerCase().slice(0, -1)} more" onclick={() => setCount(kind, n + 1)}
			><Icon name="plus" size="18px" stroke={2.6} /></button
		>
	</div>
{/snippet}

<RunPage {step} {next} {menu} {error}>
	<div class="flex min-h-0 flex-1 flex-col gap-2.5 overflow-y-auto overscroll-contain px-4 pt-6 pb-4">
		<div class="shrink-0 overflow-hidden rounded-[18px] bg-(--glass)">
			<button
				type="button"
				class="flex h-12 w-full items-center gap-2.5 px-3.5 text-left text-[15px] font-semibold text-(--fg2) shadow-[inset_0_-1px_0_rgba(255,255,255,0.06)]"
				onclick={() => (changing = true)}
			>
				<span class="flex size-7 shrink-0 items-center justify-center rounded-full bg-(--glass) text-(--fg)"><Icon name="mountain" size="15px" stroke={2.4} /></span>
				<span class="min-w-0 flex-1 truncate"
					><b class="text-(--fg)">{disciplines[discipline]}</b> · {settings[setting]}{discipline === 'boulder' ? '' : ` · ${ropes[rope]}`}</span
				>
				<span class="flex items-center gap-1">Change<Icon name="chevron-down" size="16px" stroke={2.6} /></span>
			</button>
			<div class="px-3.5 pt-3 pb-3.5">
				<p class="flex justify-between font-[family-name:var(--font-digits)] text-sm font-bold tracking-[0.1em] text-(--fg2)">
					<span>CLIMB {ended.length + 1}{system ? ` · ${system}` : ''}</span>
					{#if ended.length}<span>{sends} OF {ended.length} SENT</span>{/if}
				</p>
				<div class="mt-2">{@render strip(cur.grade, gradeList, pick, true)}</div>
				<button type="button" class="mt-1.5 flex min-h-11 w-full flex-wrap items-center gap-2 py-1 text-left" aria-label="{cur.goes.length} {cur.goes.length === 1 ? 'go' : 'goes'}, edit" onclick={() => (editing = cur.id)}>
					<span class="mr-1 text-[15px] font-bold text-(--fg2)">Goes</span>
					{#each cur.goes as g, i (i)}{@render goDot(g, i + 1)}{/each}
					<span
						class="flex size-[30px] shrink-0 items-center justify-center rounded-full border-[1.5px] border-dashed border-(--fg2)/60 font-[family-name:var(--font-digits)] text-[15px] font-bold text-(--fg2)"
						aria-hidden="true">{cur.goes.length + 1}</span
					>
					{#if cur.goes.length}<span class="ml-auto flex items-center gap-0.5 text-sm font-bold text-(--fg2)">Edit<Icon name="chevron-right" size="16px" stroke={2.6} /></span>{/if}
				</button>
				<div class="mt-3 grid grid-cols-2 gap-2.5">
					<button type="button" class="flex h-14 items-center justify-center gap-2 rounded-full bg-(--glass) text-[17px] font-bold" onclick={() => go('attempt')}>
						<span class="flex size-8 items-center justify-center rounded-full bg-white/10"><Icon name="plus" size="16px" stroke={2.8} /></span>Attempt
					</button>
					<button type="button" class="flex h-14 items-center justify-center gap-2 rounded-full bg-rest-on-hero text-[17px] font-bold text-on-tint" onclick={() => go('send')}>
						<span class="flex size-8 items-center justify-center rounded-full bg-black/10"><Icon name="check" size="16px" stroke={3} /></span>Send
					</button>
				</div>
				<div class="mt-2.5 flex flex-col gap-2">
					{#each [['focus', 'target', 'Focus'], ['notes', 'notepad', 'Note']] as const as [key, icon, label] (key)}
						<button
							type="button"
							class="flex h-11 min-w-0 items-center gap-2 rounded-full px-1.5 text-left text-[15px] font-semibold shadow-[inset_0_0_0_1.5px_color-mix(in_srgb,var(--fg)_16%,transparent)] {cur[key]
								? 'text-(--fg)'
								: 'text-(--fg2)'}"
							aria-label={cur[key] ? `${label}: ${cur[key]}` : `Add a ${label.toLowerCase()}`}
							onclick={() => (editing = cur.id)}
						>
							<span class="flex size-8 shrink-0 items-center justify-center rounded-full bg-(--glass)"><Icon name={icon} size="16px" stroke={2.4} /></span>
							<span class="min-w-0 flex-1 truncate pr-2">{cur[key] || label}</span>
						</button>
					{/each}
				</div>
			</div>
			{#if cur.goes.length}
				<button type="button" class="flex h-[52px] w-full items-center justify-center gap-2 text-[15px] font-bold shadow-[inset_0_1px_0_rgba(255,255,255,0.06)]" onclick={addClimb}>
					<span class="flex size-7 items-center justify-center rounded-full bg-(--glass)"><Icon name="plus" size="16px" stroke={2.6} /></span>Add climb
				</button>
			{/if}
		</div>

		{#if ended.length}
			<div class="flex items-baseline justify-between px-1 pt-1">
				<b class="text-[15px]">This step</b>
				<span class="text-sm font-semibold text-(--fg2)">{ended.length} {ended.length === 1 ? 'climb' : 'climbs'} · {sends} sent</span>
			</div>
			<ol class="shrink-0 overflow-hidden rounded-[18px] bg-(--glass)">
				{#each ended.toReversed() as c, i (c.id)}
					<li class={i ? 'shadow-[inset_0_1px_0_rgba(255,255,255,0.06)]' : ''}>
						<button type="button" class="flex min-h-16 w-full items-center gap-3 py-2.5 pr-3.5 pl-2.5 text-left" onclick={() => (editing = c.id)}>
							<span
								class="flex h-11 min-w-12 shrink-0 items-center justify-center rounded-xl px-1.5 font-[family-name:var(--font-digits)] text-xl font-bold {c.sent
									? 'bg-rest-on-hero text-on-tint'
									: 'bg-white/8 shadow-[inset_0_0_0_1.5px_color-mix(in_srgb,var(--fg)_30%,transparent)]'}">{c.grade ?? '–'}</span
							>
							<span class="min-w-0 flex-1">
								<b class="block text-base">{word(c)}</b>
								{#if c.notes || c.focus}<span class="block truncate text-sm font-semibold text-(--fg2)">{c.notes ?? `Focus: ${c.focus}`}</span>{/if}
							</span>
							<span class="flex gap-1" aria-hidden="true">
								{#each c.goes as g, j (j)}<i class="size-[9px] rounded-full {g === 'send' ? 'bg-rest-on-hero' : 'bg-(--fg2)/45'}"></i>{/each}
							</span>
						</button>
					</li>
				{/each}
			</ol>
		{/if}
	</div>

	<Sheet bind:open={() => editing !== null, (v) => !v && (editing = null)} title="Climb {editingNumber}" eyebrow={editingClimb ? word(editingClimb) : 'Now'}>
		{@const c = editingClimb ?? cur}
		<div class="flex flex-col gap-4 px-1">
			{#if editingClimb}
				<div class="flex flex-col gap-1.5">
					<span class="text-xs font-semibold text-ink-2">Grade</span>
					{@render strip(editingClimb.grade, gradesFor(editingClimb.discipline), (g) => setGrade(editingClimb!, g))}
				</div>
			{/if}
			{@render count('attempt', 'Attempts', c.goes.filter((g) => g === 'attempt').length)}
			{@render count('send', 'Sends', c.goes.filter((g) => g === 'send').length)}
			<label class="flex flex-col gap-1.5 text-xs font-semibold text-ink-2"
				>Focus
				<textarea
					class="min-h-20 resize-none rounded-[18px] bg-surface p-3.5 text-[15px] font-semibold text-ink outline-none placeholder:text-ink-3"
					placeholder="What to work on"
					value={c.focus ?? ''}
					onchange={(e) => setNote('focus', e.currentTarget.value)}
				></textarea>
			</label>
			<label class="flex flex-col gap-1.5 text-xs font-semibold text-ink-2"
				>Note
				<textarea
					class="min-h-24 resize-none rounded-[18px] bg-surface p-3.5 text-[15px] font-semibold text-ink outline-none placeholder:text-ink-3"
					placeholder="How did it go?"
					value={c.notes ?? ''}
					onchange={(e) => setNote('notes', e.currentTarget.value)}
				></textarea>
			</label>
			{#if editingClimb}
				<button type="button" class="flex h-12 items-center justify-center gap-2 rounded-full bg-surface text-[15px] font-bold text-bad" onclick={() => remove(editingClimb!)}>
					<Icon name="trash-2" size="17px" stroke={2.4} />Delete climb
				</button>
			{/if}
		</div>
	</Sheet>

	<Sheet bind:open={changing} title="Climbing" eyebrow={step.name}>
		<div class="flex flex-col gap-3 px-1">
			<Segmented label="Discipline" items={Object.entries(disciplines).map(([d, label]) => ({ label, on: discipline === d, onclick: () => setDiscipline(d as Discipline) }))} />
			<Segmented label="Indoor or outdoor" items={Object.entries(settings).map(([v, label]) => ({ label, on: setting === v, onclick: () => setSetting(v as keyof typeof settings) }))} />
			{#if discipline !== 'boulder'}
				<Segmented label="Rope" items={Object.entries(ropes).map(([v, label]) => ({ label, on: rope === v, onclick: () => setRope(v as keyof typeof ropes) }))} />
			{/if}
		</div>
	</Sheet>

	<div style="--above-bar: calc(env(safe-area-inset-bottom) + {last ? 118 : 212}px)">
		<Toast bind:message={toast} action="Undo" onaction={undo} />
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
