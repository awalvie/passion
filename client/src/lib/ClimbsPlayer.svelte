<script lang="ts">
	import { untrack } from 'svelte';
	import { loadGrades, scaleFor, type Grades } from './grades';
	import Icon from './Icon.svelte';
	import { newId } from './id';
	import type { MenuItem } from './Menu.svelte';
	import type { Account } from './plan';
	import { climbsOf, isFinished, type Discipline, type Go, type RunStep } from './run';
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

	const logged = $derived(climbsOf(openRun.run!, step.id));

	let info = $state<{ grades: Grades; account: Account } | null>(null);
	$effect(() => {
		loadGrades()
			.then((g) => (info = g))
			.catch(() => {});
	});

	const disciplines: Record<Discipline, string> = { boulder: 'Boulder', sport: 'Sport', trad: 'Trad' };
	const settings = { indoor: 'Indoor', outdoor: 'Outdoor' };

	// The climb on the card. It reaches the server at its first go, and stays
	// the current one until "Add climb", so after a reload the last climb of
	// an unfinished step is current again.
	type Current = { id: string; grade: string | null; goes: Go[] };
	let cur = $state<Current>({ id: '', grade: null, goes: [] });
	$effect.pre(() => {
		const id = step.id;
		untrack(() => {
			const end = climbsOf(openRun.run!, id).at(-1);
			cur =
				end && !isFinished(step)
					? { id: end.id, grade: end.grade, goes: [...end.goes] }
					: { id: newId(), grade: end?.grade ?? null, goes: [] };
		});
	});

	// The bar takes the last climb's discipline and setting.
	const base = $derived(logged.at(-1));
	const discipline = $derived<Discipline>(base?.discipline ?? 'boulder');
	const setting = $derived(base?.setting ?? 'indoor');
	const scale = $derived(info ? scaleFor(info.grades, info.account, discipline === 'boulder') : undefined);
	const gradeList = $derived([...(scale?.grades ?? []), ...(info?.grades.ungraded ?? [])]);

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

	function save(c: Current) {
		const had = logged.find((l) => l.id === c.id);
		openRun.putClimb(step, c.id, {
			step: step.id,
			position: had?.position ?? Math.max(-1, ...logged.map((l) => l.position)) + 1,
			discipline: had?.discipline ?? discipline,
			setting: had?.setting ?? setting,
			board: had?.board ?? null,
			rope_style: had?.rope_style ?? (discipline === 'boulder' ? null : (base?.rope_style ?? 'lead')),
			grade: c.grade,
			grade_system: systemFor(discipline, c.grade),
			goes: [...c.goes],
			seconds: had?.seconds ?? null,
			stars: had?.stars ?? null,
			focus: had?.focus ?? null,
			notes: had?.notes ?? null
		});
	}

	function go(kind: Go) {
		cur.goes.push(kind);
		save(cur);
	}

	// A second tap on the picked grade clears it: a climb may have no grade.
	function pick(g: string) {
		cur.grade = cur.grade === g ? null : g;
		if (cur.goes.length) save(cur);
	}

	function addClimb() {
		cur = { id: newId(), grade: cur.grade, goes: [] };
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

<RunPage {step} {next} {menu} {error}>
	<div class="flex min-h-0 flex-1 flex-col gap-2.5 overflow-y-auto overscroll-contain px-4 pt-6 pb-4">
		<div class="shrink-0 overflow-hidden rounded-[18px] bg-(--glass)">
			<div class="flex h-12 items-center gap-2.5 px-3.5 text-[15px] font-semibold text-(--fg2) shadow-[inset_0_-1px_0_rgba(255,255,255,0.06)]">
				<span class="flex size-7 shrink-0 items-center justify-center rounded-full bg-(--glass) text-(--fg)"><Icon name="mountain" size="15px" stroke={2.4} /></span>
				<span class="min-w-0 flex-1 truncate"><b class="text-(--fg)">{disciplines[discipline]}</b> · {settings[setting]}</span>
			</div>
			<div class="px-3.5 pt-3 pb-3.5">
				<p class="flex justify-between font-[family-name:var(--font-digits)] text-sm font-bold tracking-[0.1em] text-(--fg2)">
					<span>CLIMB {ended.length + 1}{system ? ` · ${system}` : ''}</span>
					{#if ended.length}<span>{sends} OF {ended.length} SENT</span>{/if}
				</p>
				<div
					class="relative -mx-1 mt-2 flex h-[76px] items-center gap-1.5 overflow-x-auto px-1 py-0.5 [scrollbar-width:none] [mask-image:linear-gradient(90deg,transparent,#000_24px,#000_calc(100%-24px),transparent)]"
					use:center={[cur.grade, gradeList.length]}
					role="group"
					aria-label="Grade"
				>
					{#each gradeList as g (g)}
						<button
							type="button"
							class="shrink-0 rounded-xl px-2.5 font-[family-name:var(--font-digits)] font-bold {cur.grade === g
								? 'h-[72px] min-w-[92px] rounded-2xl bg-(--fg) text-[46px] text-on-tint'
								: 'h-14 min-w-14 bg-black/20 text-2xl text-(--fg2)'}"
							aria-pressed={cur.grade === g}
							onclick={() => pick(g)}>{g}</button
						>
					{/each}
				</div>
				<div class="mt-3 flex min-h-9 flex-wrap items-center gap-2">
					<span class="mr-1 text-[15px] font-bold text-(--fg2)">Goes</span>
					{#each cur.goes as g, i (i)}{@render goDot(g, i + 1)}{/each}
					<span
						class="flex size-[30px] shrink-0 items-center justify-center rounded-full border-[1.5px] border-dashed border-(--fg2)/60 font-[family-name:var(--font-digits)] text-[15px] font-bold text-(--fg2)"
						aria-hidden="true">{cur.goes.length + 1}</span
					>
				</div>
				<div class="mt-3 grid grid-cols-2 gap-2.5">
					<button type="button" class="flex h-14 items-center justify-center gap-2 rounded-full bg-(--glass) text-[17px] font-bold" onclick={() => go('attempt')}>
						<span class="flex size-8 items-center justify-center rounded-full bg-white/10"><Icon name="plus" size="16px" stroke={2.8} /></span>Attempt
					</button>
					<button type="button" class="flex h-14 items-center justify-center gap-2 rounded-full bg-rest-on-hero text-[17px] font-bold text-on-tint" onclick={() => go('send')}>
						<span class="flex size-8 items-center justify-center rounded-full bg-black/10"><Icon name="check" size="16px" stroke={3} /></span>Send
					</button>
				</div>
			</div>
			{#if cur.goes.length}
				<button type="button" class="flex h-[52px] w-full items-center justify-center gap-2 text-[15px] font-bold shadow-[inset_0_1px_0_rgba(255,255,255,0.06)]" onclick={addClimb}>
					<span class="flex size-7 items-center justify-center rounded-full bg-(--glass)"><Icon name="plus" size="16px" stroke={2.6} /></span>Add climb
				</button>
			{/if}
		</div>
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
