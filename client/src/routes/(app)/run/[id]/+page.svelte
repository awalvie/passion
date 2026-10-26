<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import type { Snapshot } from '@sveltejs/kit';
	import { describe } from '$lib/api';
	import Button from '$lib/Button.svelte';
	import FormError from '$lib/FormError.svelte';
	import Icon, { type IconName } from '$lib/Icon.svelte';
	import JournalSheet from '$lib/JournalSheet.svelte';
	import Menu from '$lib/Menu.svelte';
	import {
		currentStop,
		climbsOf,
		isFinished,
		isSkipped,
		sectionMarks,
		sectionShare,
		setsOf,
		stepsOf,
		stopHref,
		type RunChoice,
		type RunSection,
		type RunStep,
		type SectionMark
	} from '$lib/run';
	import RunField from '$lib/RunField.svelte';
	import { openRun } from '$lib/runState.svelte';
	import { choiceMeta, stepMeta, toStep, type Step } from '$lib/template';
	import { loadExercises, type Exercise } from '$lib/exercise';
	import ExercisePicker from '$lib/ExercisePicker.svelte';
	import SessionDot from '$lib/SessionDot.svelte';
	import Notes from '$lib/Notes.svelte';
	import Section from '$lib/Section.svelte';
	import Sheet from '$lib/Sheet.svelte';

	const run = $derived(openRun.run!);
	const steps = $derived(stepsOf(run));
	const finished = $derived(steps.filter((s) => isFinished(s) && s.status !== 'skipped').length);
	const stop = $derived(currentStop(run));
	const current = $derived(stop?.step);
	const section = $derived(stop ? run.sections.findIndex((s) => s.items.includes(stop)) : -1);

	let error = $state('');

	// The options marked in each choice until Use saves them. The run page and
	// the picks sheet share them, so a closed sheet keeps its marks.
	let selected = $state<Record<string, number[]>>({});

	function toggle(id: string, k: number) {
		const now = selected[id] ?? [];
		selected[id] = now.includes(k) ? now.filter((x) => x !== k) : [...now, k];
	}

	function choose(id: string, k: number) {
		if (!selected[id]?.includes(k)) toggle(id, k);
	}

	const enough = (c: RunChoice) => (selected[c.id]?.length ?? 0) >= Math.max(1, c.pick);

	let library = $state<Exercise[]>([]);
	let picking = $state(false);
	let into = $state<number>();
	// The exercise shown before it is added. Each opening remounts the picker,
	// so it starts with an empty search.
	let candidate = $state<Exercise | null>(null);
	let opened = $state(0);

	async function openLibrary(section?: number) {
		error = '';
		try {
			if (!library.length) {
				library = (await loadExercises()).filter((e) => !e.retired_at);
			}
			into = section;
			candidate = null;
			opened++;
			picking = true;
		} catch (e) {
			error = describe(e, (f) => f);
		}
	}

	// The picker sends a missing exercise to the library in another tab, so
	// the list reloads when the user comes back to this one.
	async function reload() {
		if (!library.length) return;
		try {
			library = (await loadExercises(true)).filter((e) => !e.retired_at);
		} catch {
			/* keep the list it has */
		}
	}

	function pick(id: string, options: Step[]) {
		openRun.pick(id, options);
		delete selected[id];
	}

	// Pick now lists every pick still to make in one sheet. A choice that asks
	// for all its options starts with them marked. Use saves each choice with
	// enough marked, and one left short stays on the page for later.
	let listing = $state(false);
	let peek = $state<{ choice: RunChoice; k: number } | null>(null);
	const peeked = $derived(peek && peek.choice.options[peek.k]);

	function openPicks() {
		for (const c of unpicked) {
			if (c.pick === c.options.length && !selected[c.id]?.length) selected[c.id] = c.options.map((_, k) => k);
		}
		peek = null;
		listing = true;
	}

	function useMarked() {
		for (const c of unpicked.filter(enough)) pick(c.id, selected[c.id].map((k) => c.options[k]));
		listing = false;
	}

	// The sheet keeps the last option it showed, so it does not go blank while
	// it slides down.
	let previewing = $state(false);
	let shown = $state<{ choice: RunChoice; k: number }>();
	const option = $derived(shown?.choice.options[shown.k]);

	function preview(choice: RunChoice, k: number) {
		shown = { choice, k };
		previewing = true;
	}

	function pickShown() {
		const { choice, k } = shown!;
		if (choice.pick === 1) pick(choice.id, [choice.options[k]]);
		else choose(choice.id, k);
		previewing = false;
	}

	async function discard() {
		error = '';
		try {
			await openRun.discard();
		} catch (e) {
			error = describe(e, (f) => f);
		}
	}

	function mark(s: RunStep): 'done' | 'skipped' | 'now' | 'todo' {
		if (s.status === 'skipped') return 'skipped';
		if (isFinished(s)) return 'done';
		return s.id === current?.id ? 'now' : 'todo';
	}

	const marks = $derived(sectionMarks(run, section));

	// Skip section skips the steps nothing was logged on, and the choices not
	// picked from; a step under way is left for its own Skip.
	const untouched = (s: RunSection) =>
		stepsIn(s).filter((st) => st.status === null && !setsOf(run, st.id).length && !climbsOf(run, st.id).length);
	const stepsIn = (s: RunSection) => s.items.flatMap((it) => (it.step ? [it.step] : []));
	const choicesIn = (s: RunSection) => s.items.flatMap((it) => (it.choice ? [it.choice] : []));
	function sectionMenu(s: RunSection) {
		if (run.finished_at) return [];
		const steps = untouched(s);
		const choices = choicesIn(s).filter((c) => !c.skipped);
		const skippedSteps = stepsIn(s).filter((st) => st.status === 'skipped');
		const skippedChoices = choicesIn(s).filter((c) => c.skipped);
		const skip = () => {
			steps.forEach((st) => openRun.skip(st));
			choices.forEach((c) => openRun.skipChoice(c));
		};
		const undo = () => {
			skippedSteps.forEach((st) => openRun.unskip(st));
			skippedChoices.forEach((c) => openRun.unskipChoice(c));
		};
		return [
			...(steps.length || choices.length ? [{ label: 'Skip section', onclick: skip }] : []),
			...(skippedSteps.length || skippedChoices.length ? [{ label: 'Undo skips', onclick: undo }] : [])
		];
	}

	function caption(s: RunSection, m: SectionMark): string {
		if (!s.items.length) return 'No exercises';
		if (m === 'skipped') return 'Skipped';
		if (m === 'done') {
			const skipped = s.items.filter(isSkipped);
			const name = skipped[0]?.step?.name ?? skipped[0]?.choice?.name;
			const tail = skipped.length === 1 ? `${name} skipped` : skipped.length ? `${skipped.length} skipped` : '';
			return [`${s.items.length - skipped.length} done`, tail].filter(Boolean).join(' · ');
		}
		const only = s.items[0];
		if (s.items.length === 1) return only.step ? stepMeta(only.step) : choiceMeta(only.choice!) || '1 exercise';
		return `${s.items.length} exercises`;
	}

	// A tap opens or closes a section until the current one moves on, and then
	// the new current section opens.
	let chosen = $state<Record<number, boolean>>({});
	$effect.pre(() => {
		void section;
		chosen = {};
	});
	// A link to a choice opens its section too.
	const asked = $derived(page.url.hash.startsWith('#pick-') ? page.url.hash.slice(6) : '');
	const askedIn = $derived(run.sections.findIndex((s) => s.items.some((it) => it.choice?.id === asked)));
	// A step moved into another section opens that section, so it does not
	// vanish into a folded one.
	let moved = $state(-1);
	// With every step done, the last section stays open for its Add exercise.
	const isOpen = (i: number) =>
		chosen[i] ?? (i === section || i === askedIn || i === moved || (section < 0 && i === run.sections.length - 1 && !run.finished_at));

	function landed(id: string) {
		moved = run.sections.findIndex((s) => s.items.some((it) => it.step?.id === id));
		delete chosen[moved];
	}

	function shift(id: string, by: -1 | 1) {
		openRun.shiftStep(id, by);
		landed(id);
	}

	// A hold on a step not yet started lifts it; it then follows the finger,
	// and the drop puts it where the lime line shows. A move before the hold
	// ends is a scroll, so the hold gives up.
	let lifted = $state<RunStep | null>(null);
	let at = $state({ x: 0, y: 0 });
	// Where the step would land: before item `j` of section `s`, counted
	// before it leaves its place.
	let over = $state<{ s: number; j: number } | null>(null);
	let hold: ReturnType<typeof setTimeout> | undefined;
	let start = { x: 0, y: 0 };
	// The click that ends a drag lands on the row it started from; it must not
	// open that step.
	let droppedAt = 0;

	$effect(() => {
		const stop = (e: TouchEvent) => lifted && e.preventDefault();
		window.addEventListener('touchmove', stop, { passive: false });
		return () => window.removeEventListener('touchmove', stop);
	});

	// A finger held near the top or bottom of the list scrolls it.
	$effect(() => {
		if (!lifted) return;
		const t = setInterval(() => {
			const box = scroller.getBoundingClientRect();
			if (at.y < box.top + 64) scroller.scrollTop -= 8;
			else if (at.y > box.bottom - 64) scroller.scrollTop += 8;
		}, 16);
		return () => clearInterval(t);
	});

	function grab(e: PointerEvent, step: RunStep) {
		if (e.button > 0 || (e.target as Element).closest('button')) return;
		const el = e.currentTarget as Element;
		const id = e.pointerId;
		start = { x: e.clientX, y: e.clientY };
		at = start;
		clearTimeout(hold);
		hold = setTimeout(() => {
			lifted = step;
			over = null;
			el.setPointerCapture(id);
			navigator.vibrate?.(10);
		}, 300);
	}

	function follow(e: PointerEvent) {
		if (!lifted) {
			if (Math.hypot(e.clientX - start.x, e.clientY - start.y) > 8) clearTimeout(hold);
			return;
		}
		at = { x: e.clientX, y: e.clientY };
		const slot = document.elementFromPoint(e.clientX, e.clientY)?.closest<HTMLElement>('[data-slot]');
		if (!slot) {
			over = null;
			return;
		}
		const [s, j] = slot.dataset.slot!.split(':').map(Number);
		const r = slot.getBoundingClientRect();
		over = { s, j: slot.dataset.row !== undefined && e.clientY > r.top + r.height / 2 ? j + 1 : j };
	}

	function drop() {
		clearTimeout(hold);
		const step = lifted;
		const to = over;
		lifted = null;
		over = null;
		if (!step) return;
		droppedAt = performance.now();
		if (!to) return;
		openRun.dropStep(step.id, to.s, to.j);
		landed(step.id);
	}

	function cancel() {
		clearTimeout(hold);
		lifted = null;
		over = null;
	}

	// Choices still to pick, asked about until a step is logged or skipped.
	let later = $state(false);
	const unpicked = $derived(
		run.sections.flatMap((s) => s.items.flatMap((it) => (it.choice && it.choice.pick > 0 && !it.choice.skipped ? [it.choice] : [])))
	);
	const remind = $derived(!later && !run.finished_at && steps.every((s) => s.status === null) && unpicked.length > 0);

	// The page scrolls inside the field, so coming back from a step puts the
	// list back where it was.
	let scroller: HTMLElement;
	export const snapshot: Snapshot<number> = {
		capture: () => scroller.scrollTop,
		restore: (y) => (scroller.scrollTop = y)
	};

	let naming = $state(false);
	let sectionName = $state('');
	let journaling = $state(false);

	function addSection(e: SubmitEvent) {
		e.preventDefault();
		openRun.addSection(sectionName.trim());
		sectionName = '';
		naming = false;
	}
</script>

{#snippet tile(icon: IconName)}
	<span class="flex size-7 shrink-0 items-center justify-center rounded-full bg-(--glass) text-on-hero"><Icon name={icon} size="16px" stroke={2.6} /></span>
{/snippet}

{#snippet howTo(step: Step)}
	{@const video = step.media.find((md) => md.url)}
	{@const thumb = step.media.find((md) => md.url && md.thumb_url)}
	<p class="-mt-1 text-xs font-semibold text-ink-2">{stepMeta(step)}</p>
	{#if thumb}
		<a
			href={thumb.url}
			target="_blank"
			rel="noopener"
			class="relative mt-3.5 block aspect-video max-w-full overflow-hidden rounded-[18px] bg-well"
			aria-label="Watch {step.name}"
		>
			<img src={thumb.thumb_url} alt="" class="size-full object-cover" />
			<span class="absolute top-1/2 left-1/2 flex size-14 -translate-x-1/2 -translate-y-1/2 items-center justify-center rounded-full bg-white/90 pl-0.5 text-on-tint">
				<Icon name="play" size="1.5rem" />
			</span>
		</a>
	{:else if video}
		<a href={video.url} target="_blank" rel="noopener" class="mt-3.5 flex items-center gap-2.5 text-[15px] font-bold">
			<span class="flex size-8 items-center justify-center rounded-full bg-well pl-0.5"><Icon name="play" size="1rem" /></span>
			Watch video
		</a>
	{/if}
	{#if step.notes?.trim()}
		<div class="mt-3.5 text-[15px] font-semibold"><Notes text={step.notes} /></div>
	{/if}
{/snippet}

{#snippet line()}
	<li aria-hidden="true" class="mx-1 my-0.5 h-[3px] rounded-full bg-tint"></li>
{/snippet}

{#snippet optionRows(c: RunChoice, onpreview: (k: number) => void)}
	<div>
		{#each c.options as o, k (k)}
			{@const on = !!selected[c.id]?.includes(k)}
			{@const thumb = o.media.find((md) => md.thumb_url)?.thumb_url}
			<div class="flex items-center gap-3 px-1 list-row">
				<button
					type="button"
					class="relative flex size-7 shrink-0 items-center justify-center rounded-full before:absolute before:-inset-2 before:content-[''] {on
						? 'bg-tint text-on-tint'
						: 'shadow-[inset_0_0_0_2px_var(--ink-3)]'}"
					aria-pressed={on}
					aria-label="Pick {o.name}"
					onclick={() => toggle(c.id, k)}
				>
					{#if on}
						<Icon name="check" size="1rem" stroke={3} />
					{/if}
				</button>
				<button type="button" class="flex min-h-[60px] min-w-0 flex-1 items-center gap-3 py-2 text-left" onclick={() => onpreview(k)}>
					{#if thumb}
						<img src={thumb} alt="" class="size-11 shrink-0 rounded-xl object-cover" />
					{/if}
					<span class="min-w-0 flex-1">
						<span class="block truncate text-[15px] font-bold">{o.name}</span>
						<span class="mt-0.5 block truncate text-xs font-semibold text-ink-2">{stepMeta(o)}</span>
					</span>
					<span class="text-ink-3"><Icon name="chevron-right" size="1.125rem" /></span>
				</button>
			</div>
		{/each}
	</div>
{/snippet}

{#snippet useFooter()}
	{@const ready = unpicked.filter(enough).length}
	<Button disabled={!ready} onclick={useMarked}>{ready > 1 ? `Use ${ready} picks` : 'Use this pick'}</Button>
{/snippet}

<svelte:head><title>{run.name}</title></svelte:head>
<svelte:window onfocus={reload} />

<RunField
	label={run.name}
	back={run.finished_at
		? { href: `/history/${run.id}`, label: 'History', swipe: true }
		: { href: '/', label: 'Today', swipe: true }}
>
	{#snippet end()}
		<Menu
			look="bg-(--pill) text-on-hero"
			items={[
				...(run.finished_at ? [] : [{ label: 'Write in journal', onclick: () => (journaling = true) }]),
				{ label: run.finished_at ? 'Edit session journal' : 'Finish session', onclick: () => goto(`/run/${run.id}/finish`) },
				{ label: 'Discard session', danger: true, onclick: discard }
			]}
		/>
	{/snippet}

	<div class="min-h-0 flex-1 overflow-y-auto overscroll-contain" bind:this={scroller}>
		<h1 class="mt-3 mb-4 px-5 text-center text-[28px] leading-[1.08] font-extrabold tracking-[-0.02em] break-words text-balance">
			{run.name}
		</h1>
		<div class="px-4">
			<FormError message={error} />
		</div>
		<div class="flex flex-col px-4 {run.finished_at ? 'pb-[calc(env(safe-area-inset-bottom)+1.5rem)]' : 'pb-6'}">
			<section class="rounded-3xl bg-(--glass) px-5 pt-4 pb-[18px]">
				<p class="flex flex-wrap justify-between gap-x-3 font-[family-name:var(--font-digits)] text-sm font-bold tracking-[0.1em] text-(--fg2)">
					<span>{finished} OF {steps.length} DONE</span>
					<span>{run.finished_at ? 'FINISHED' : section >= 0 ? `SECTION ${section + 1} OF ${run.sections.length}` : 'ALL DONE'}</span>
				</p>
				<div
					class="mt-3.5 flex gap-1.5"
					role="progressbar"
					aria-valuemin={0}
					aria-valuemax={steps.length}
					aria-valuenow={finished}
				>
					{#each run.sections as s, i (i)}
						<span class="h-2 flex-1 overflow-hidden rounded-full {marks[i] === 'done' ? 'bg-rest-on-hero' : 'bg-well'}">
							{#if marks[i] === 'now'}
								<span class="block h-full rounded-full bg-tint" style="width: {sectionShare(s) * 100}%"></span>
							{/if}
						</span>
					{/each}
				</div>
			</section>

			{#if remind}
				<div class="mt-2.5 rounded-3xl bg-(--glass) px-4 pt-3.5 pb-4 shadow-[0_0_0_2px_var(--tint)]">
					<p class="text-[17px] font-extrabold">
						{unpicked.length === 1 ? '1 pick to make' : `${unpicked.length} picks to make`}
					</p>
					<p class="mt-0.5 truncate text-sm font-semibold text-ink-2">{unpicked.map((c) => c.name).join(' · ')}</p>
					<div class="mt-3 grid grid-cols-[1fr_1.4fr] gap-2.5">
						<button type="button" class="flex h-14 items-center justify-center rounded-full bg-(--glass) text-[15px] font-bold text-ink active:opacity-80" onclick={() => (later = true)}>Later</button>
						<Button onclick={openPicks}>Pick now</Button>
					</div>
				</div>
			{/if}

			<ol class="mt-1.5">
				{#each run.sections as s, i (i)}
					{@const m = marks[i]}
					{@const open = isOpen(i)}
					{@const actions = open ? sectionMenu(s) : []}
					<li class="relative">
						{#if i < run.sections.length - 1}
							<span
								aria-hidden="true"
								class="absolute top-[26px] -bottom-[26px] left-[14.75px] border-l-[2.5px] {m === 'done'
									? 'border-rest-on-hero/60'
									: 'border-dotted border-ink-3 opacity-70'}"
							></span>
						{/if}
						<div class="flex items-center gap-1">
							<button
								type="button"
								class="relative flex min-w-0 flex-1 items-center gap-3.5 rounded-[18px] pr-1 text-left {m === 'now' ? 'h-11' : 'h-[52px]'} {lifted && !open && over?.s === i
									? 'bg-tint/20'
									: ''}"
								data-slot="{i}:{s.items.length}"
								aria-expanded={open}
								onclick={() => (chosen[i] = !isOpen(i))}
							>
								<SessionDot mark={m} />
								<span class="min-w-0 flex-1">
									<span class="block truncate text-[15px] font-bold">{s.name}</span>
									{#if m !== 'now'}
										<span class="mt-0.5 block truncate text-xs font-semibold text-ink-2">{caption(s, m)}</span>
									{/if}
								</span>
								{#if m === 'now'}
									<span class="shrink-0 rounded-[10px] bg-tint px-2.5 py-1 text-xs font-bold text-on-tint">You are here</span>
								{:else}
									{#if m === 'done'}
										<span class="flex shrink-0 gap-1" aria-hidden="true">
											{#each s.items as it, k (k)}
												<span
													class="size-[7px] rounded-full {isSkipped(it)
														? 'border-[1.5px] border-ink-3'
														: 'bg-rest-on-hero'}"
												></span>
											{/each}
										</span>
									{/if}
									<span class="text-ink-3 transition-transform {open ? 'rotate-90' : ''}">
										<Icon name="chevron-right" size="1.125rem" />
									</span>
								{/if}
							</button>
							{#if actions.length}
								<Menu label="Actions for {s.name}" look="bg-(--glass) text-on-hero" items={actions} />
							{/if}
						</div>

						{#if open && (s.items.length || !run.finished_at)}
							<ul class="relative mb-1 ml-[46px] rounded-3xl bg-(--glass) p-2.5 pb-2">
								{#each s.items as item, j (item.step?.id ?? item.choice?.id ?? j)}
									{#if over?.s === i && over.j === j}{@render line()}{/if}
									{@const movable = !!item.step && item.step.status === null && !run.finished_at}
									<li
										class="flex items-center [&:not(:first-child)]:mt-1 list-row {lifted && lifted === item.step ? 'opacity-30' : ''} {movable
											? 'select-none [-webkit-touch-callout:none]'
											: ''}"
										data-slot="{i}:{j}"
										data-row
										onpointerdown={movable ? (e) => grab(e, item.step!) : undefined}
										onpointermove={follow}
										onpointerup={drop}
										onpointercancel={cancel}
										oncontextmenu={movable ? (e) => e.preventDefault() : undefined}
									>
										{#if item.step}
											{@const sm = mark(item.step)}
											<a
												href="/run/{run.id}/step/{item.step.id}"
												class="flex min-w-0 flex-1 items-center gap-3 px-1 py-2.5"
												draggable="false"
												onclick={(e) => performance.now() - droppedAt < 500 && e.preventDefault()}
											>
												<SessionDot mark={sm} small />
												<span class="min-w-0 flex-1">
													<span
														class="block truncate text-[15px] font-bold {sm === 'skipped'
															? 'text-ink-2 line-through'
															: sm === 'done'
																? 'text-ink-2'
																: ''}"
													>
														{item.step.name}
													</span>
													<span class="mt-0.5 block truncate text-xs font-semibold text-ink-2">{stepMeta(item.step)}</span>
													{#if sm === 'now' && item.step.sets && item.step.kind !== 'climbing'}
														{@const logged = setsOf(run, item.step.id).length}
														<span class="mt-2 flex items-center gap-1">
															{#each Array.from({ length: item.step.sets }, (_, n) => n) as n (n)}
																<span
																	class="size-2 rounded-full {n < logged
																		? 'bg-rest-on-hero'
																		: n === logged
																			? 'bg-tint'
																			: 'bg-well'}"
																></span>
															{/each}
															{#if logged < item.step.sets}
																<span class="ml-0.5 text-xs font-bold">Set {logged + 1} next</span>
															{/if}
														</span>
													{/if}
												</span>
												{#if !movable}
													<span class="text-ink-3"><Icon name="chevron-right" size="1.125rem" /></span>
												{/if}
											</a>
											{#if movable}
												{@const id = item.step.id}
												<Menu
													label="Actions for {item.step.name}"
													look="bg-(--glass) text-on-hero"
													items={[
														...(i > 0 || j > 0 ? [{ label: 'Move up', onclick: () => shift(id, -1) }] : []),
														...(i < run.sections.length - 1 || j < s.items.length - 1
															? [{ label: 'Move down', onclick: () => shift(id, 1) }]
															: []),
														{ label: 'Remove', danger: true, onclick: () => openRun.removeStep(id) }
													]}
												/>
											{/if}
										{:else if item.choice.skipped}
											<div class="flex min-w-0 flex-1 items-center gap-3 px-1 py-2.5">
												<SessionDot mark="skipped" small />
												<span class="min-w-0 flex-1">
													<span class="block truncate text-[15px] font-bold text-ink-2 line-through">{item.choice.name}</span>
													<span class="mt-0.5 block truncate text-xs font-semibold text-ink-2">{choiceMeta(item.choice)}</span>
												</span>
											</div>
										{:else}
											{@const c = item.choice}
											<div id="pick-{c.id}" class="w-full scroll-mt-20 pt-0.5 pb-1">
												<p class="px-1 pt-0.5 pb-2 text-xs font-semibold tracking-[0.06em] text-ink-2 uppercase">
													{[c.name, choiceMeta(c)].filter(Boolean).join(' · ')}
												</p>
												{@render optionRows(c, (k) => preview(c, k))}
												{#if selected[c.id]?.length}
													<div class="mt-2.5">
														<Button disabled={!enough(c)} onclick={() => pick(c.id, selected[c.id].map((k) => c.options[k]))}>
															Use {selected[c.id].length === 1 ? 'this' : 'these'}
														</Button>
													</div>
												{/if}
											</div>
										{/if}
									</li>
								{/each}
								{#if over?.s === i && over.j === s.items.length}{@render line()}{/if}
								{#if !run.finished_at}
									<li class="[&:not(:first-child)]:mt-1 list-row" data-slot="{i}:{s.items.length}">
										<button type="button" class="flex w-full items-center gap-3 px-1 py-2.5 text-[15px] font-bold text-on-hero" onclick={() => openLibrary(i)}>
											{@render tile('plus')}Add exercise
										</button>
									</li>
								{/if}
							</ul>
						{/if}
					</li>
				{/each}
			</ol>

			{#if !run.finished_at}
				<div class="mt-5 flex flex-col gap-2.5">
					{#if !run.sections.length}
						<Button variant="secondary" onclick={() => openLibrary()}>{@render tile('plus')}Add exercise</Button>
					{/if}
					<Button variant="secondary" onclick={() => (naming = true)}>{@render tile('rows-2')}Add section</Button>
				</div>
			{/if}
		</div>
	</div>

	<Sheet bind:open={picking} eyebrow={into === undefined ? undefined : run.sections[into]?.name} title={candidate?.name ?? 'Add exercise'}>
		<div hidden={!!candidate}>
			{#key opened}
				<ExercisePicker id="add-exercise" label="Exercise" exercises={library} keep pick={(e) => (candidate = e)} />
			{/key}
		</div>
		{#if candidate}
			{@render howTo(toStep(candidate))}
			<div class="mt-5 grid grid-cols-[1fr_1.4fr] gap-2.5">
				<button type="button" class="flex h-14 items-center justify-center rounded-full bg-well text-[15px] font-bold text-ink active:opacity-80" onclick={() => (candidate = null)}>Back</button>
				<Button
					onclick={() => {
						openRun.addStep(candidate!, into);
						picking = false;
					}}>Add</Button
				>
			</div>
		{/if}
	</Sheet>

	<Sheet bind:open={naming} title="Add section">
		<form onsubmit={addSection}>
			<label class="block text-xs font-semibold text-ink-2" for="section-name">Name</label>
			<input id="section-name" class="mt-1.5 input" maxlength="200" required bind:value={sectionName} />
			<div class="mt-4">
				<Button type="submit" disabled={!sectionName.trim()}>Add section</Button>
			</div>
		</form>
	</Sheet>

	<Sheet
		bind:open={listing}
		eyebrow={peek?.choice.name}
		title={peeked?.name ?? 'Picks to make'}
		onback={peek ? () => (peek = null) : undefined}
		footer={peek ? undefined : useFooter}
	>
		{#if peeked}
			{@render howTo(peeked)}
			<div class="mt-5">
				<Button
					onclick={() => {
						choose(peek!.choice.id, peek!.k);
						peek = null;
					}}>Pick this</Button
				>
			</div>
		{:else}
			<div class="flex flex-col gap-5">
				{#each unpicked as c (c.id)}
					<Section title={c.name}>
						{#snippet aside()}<span class="text-xs font-semibold text-ink-2">{choiceMeta(c)}</span>{/snippet}
						<div class="rounded-3xl bg-surface px-3 shadow-card">
							{@render optionRows(c, (k) => (peek = { choice: c, k }))}
						</div>
					</Section>
				{/each}
			</div>
		{/if}
	</Sheet>

	<JournalSheet bind:open={journaling} />

	<Sheet bind:open={previewing} eyebrow={shown?.choice.name} title={option?.name ?? ''}>
		{#if option}
			{@render howTo(option)}
			<div class="mt-5 grid grid-cols-[1fr_1.4fr] gap-2.5">
				<button type="button" class="flex h-14 items-center justify-center rounded-full bg-well text-[15px] font-bold text-ink active:opacity-80" onclick={() => (previewing = false)}>Back</button>
				<Button onclick={pickShown}>Pick this</Button>
			</div>
		{/if}
	</Sheet>

	{#if !run.finished_at && (stop || steps.length)}
		<div class="shrink-0 rounded-t-[28px] bg-(--dock) px-4 pt-5 pb-[calc(env(safe-area-inset-bottom)+1.25rem)] shadow-[inset_0_1px_0_rgba(255,255,255,0.08)]" role="group" aria-label="Now">
			{#if stop}
				{@const meta = stop.step ? stepMeta(stop.step) : choiceMeta(stop.choice!)}
				<div class="flex min-h-11 flex-col justify-center gap-0.5 px-1">
					<span class="truncate text-sm font-medium text-(--fg2)">
						<b class="font-[family-name:var(--font-digits)] font-bold tracking-[0.1em]">NOW</b>{meta ? ` · ${meta}` : ''}
					</span>
					<span class="truncate text-[17px] leading-[1.2] font-bold">{stop.step?.name ?? stop.choice!.name}</span>
				</div>
				<div class="mt-4 flex gap-2.5">
					<a href={stopHref(run.id, stop)} class="run-btn bg-tint text-on-tint">
						<span class="run-icon"><Icon name="play" size="16px" /></span>Continue
					</a>
					<a href="/run/{run.id}/finish" class="run-round" aria-label="Finish session"><Icon name="flag" size="24px" stroke={2.2} /></a>
				</div>
			{:else}
				<a href="/run/{run.id}/finish" class="run-btn bg-tint text-on-tint">
					<span class="run-icon"><Icon name="flag" size="16px" stroke={2.4} /></span>Finish session
				</a>
			{/if}
		</div>
	{/if}

	{#if lifted}
		<div
			class="pointer-events-none fixed z-50 flex w-[min(300px,80vw)] -translate-x-1/2 -translate-y-[calc(100%+16px)] scale-[1.04] items-center gap-3 rounded-[18px] bg-surface px-3.5 py-2.5 shadow-[0_0_0_1.5px_var(--tint),0_18px_40px_rgba(0,0,0,0.5)]"
			style="left: {at.x}px; top: {at.y}px"
		>
			<SessionDot mark="todo" small />
			<span class="min-w-0 flex-1">
				<span class="block truncate text-[15px] font-bold">{lifted.name}</span>
				<span class="mt-0.5 block truncate text-xs font-semibold text-ink-2">{stepMeta(lifted)}</span>
			</span>
		</div>
	{/if}
</RunField>
