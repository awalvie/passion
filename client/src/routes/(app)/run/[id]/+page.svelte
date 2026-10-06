<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { describe, request } from '$lib/api';
	import Button from '$lib/Button.svelte';
	import FormError from '$lib/FormError.svelte';
	import Icon from '$lib/Icon.svelte';
	import Menu from '$lib/Menu.svelte';
	import NavBar from '$lib/NavBar.svelte';
	import { currentStop, isFinished, secondsSince, setsOf, stepsOf, stopHref, type RunSection, type RunStep } from '$lib/run';
	import { openRun } from '$lib/runState.svelte';
	import { choiceMeta, stepMeta, type Step } from '$lib/template';
	import type { Exercise } from '$lib/exercise';
	import ExercisePicker from '$lib/ExercisePicker.svelte';
	import SessionDot from '$lib/SessionDot.svelte';
	import Sheet from '$lib/Sheet.svelte';

	const run = $derived(openRun.run!);
	const steps = $derived(stepsOf(run));
	const finished = $derived(steps.filter(isFinished).length);
	const stop = $derived(currentStop(run));
	const current = $derived(stop?.step);
	const section = $derived(stop ? run.sections.findIndex((s) => s.items.includes(stop)) : -1);

	let now = $state(Date.now());
	$effect(() => {
		const tick = setInterval(() => (now = Date.now()), 1000);
		return () => clearInterval(tick);
	});
	const minutes = $derived(
		Math.floor((run.finished_at ? (run.elapsed_seconds ?? 0) : secondsSince(run.started_at, now)) / 60)
	);

	let error = $state('');

	let choosing = $state<string | null>(null);
	let picked = $state<number[]>([]);

	function toggle(id: string, k: number) {
		if (choosing !== id) {
			choosing = id;
			picked = [];
		}
		picked = picked.includes(k) ? picked.filter((x) => x !== k) : [...picked, k];
	}

	let library = $state<Exercise[]>([]);
	let picking = $state(false);
	let into = $state<number>();

	async function openLibrary(section?: number) {
		error = '';
		try {
			if (!library.length) {
				const { exercises } = await request<{ exercises: Exercise[] }>('GET', '/api/v1/exercises');
				library = exercises.filter((e) => !e.retired_at);
			}
			into = section;
			picking = true;
		} catch (e) {
			error = describe(e, (f) => f);
		}
	}

	function pick(id: string, options: Step[]) {
		openRun.pick(id, options);
		choosing = null;
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

	type SectionMark = 'done' | 'now' | 'todo';

	const marks = $derived(
		run.sections.map((s, i): SectionMark => {
			if (i === section) return 'now';
			return s.items.length && s.items.every((it) => it.step && isFinished(it.step)) ? 'done' : 'todo';
		})
	);
	const sectionsDone = $derived(marks.filter((m) => m === 'done').length);

	function share(s: RunSection): number {
		const done = s.items.filter((it) => it.step && isFinished(it.step)).length;
		return s.items.length ? done / s.items.length : 0;
	}

	function caption(s: RunSection, m: SectionMark): string {
		if (!s.items.length) return 'No exercises';
		if (m === 'done') {
			const skipped = s.items.filter((it) => it.step?.status === 'skipped');
			const tail =
				skipped.length === 1 ? `${skipped[0].step!.name} skipped` : skipped.length ? `${skipped.length} skipped` : '';
			return [`${s.items.length - skipped.length} done`, tail].filter(Boolean).join(' · ');
		}
		const only = s.items[0];
		if (s.items.length === 1) return only.step ? stepMeta(only.step) : choiceMeta(only.choice!);
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
	const isOpen = (i: number) => chosen[i] ?? (i === section || i === askedIn);

	let editing = $state(false);

	let naming = $state(false);
	let sectionName = $state('');

	function addSection(e: SubmitEvent) {
		e.preventDefault();
		openRun.addSection(sectionName.trim());
		sectionName = '';
		naming = false;
	}
</script>

{#snippet remove(label: string, onclick: () => void)}
	<button
		type="button"
		class="relative flex size-7 shrink-0 items-center justify-center rounded-full bg-well text-ink before:absolute before:-inset-2 before:content-[''] active:opacity-70"
		aria-label={label}
		{onclick}
	>
		<svg viewBox="0 0 24 24" class="size-4" fill="none" stroke="currentColor" stroke-width="2.6" stroke-linecap="round" aria-hidden="true"><path d="M6 12h12" /></svg>
	</button>
{/snippet}

<svelte:head><title>{run.name}</title></svelte:head>

{#if editing}
	<NavBar title="Edit session">
		{#snippet actions()}
			<button
				type="button"
				class="flex h-11 items-center rounded-full bg-tint px-4 text-[15px] font-bold text-on-tint shadow-tint active:opacity-80"
				onclick={() => (editing = false)}
			>
				Done
			</button>
		{/snippet}
	</NavBar>
{:else}
	<NavBar title={run.name} back={{ href: '/', label: 'Today' }}>
		{#snippet actions()}
			<button
				type="button"
				class="flex h-11 items-center rounded-full bg-surface px-4 text-[15px] font-bold text-ink shadow-card-sm active:opacity-80"
				onclick={() => {
					choosing = null;
					editing = true;
				}}
			>
				Edit
			</button>
			<Menu
				items={[
					{ label: 'Finish session', onclick: () => goto(`/run/${run.id}/finish`) },
					{ label: 'Discard session', danger: true, onclick: discard }
				]}
			/>
		{/snippet}
	</NavBar>
{/if}

<div class="px-4">
	<FormError message={error} />
</div>

{#if editing}
	<div class="px-4 pb-[calc(env(safe-area-inset-bottom)+2rem)]">
		<p class="px-1 pt-1 text-xs font-semibold text-ink-2">Remove with −. Done exercises keep their sets.</p>
		{#each run.sections as s, i (i)}
			<div class="flex items-baseline gap-2.5 px-1 pt-[18px] pb-2">
				<h2 class="min-w-0 truncate text-xl leading-[1.15] font-bold tracking-[-0.01em]">{s.name}</h2>
				<button
					type="button"
					class="ml-auto flex shrink-0 items-center gap-1 text-[15px] font-bold text-ink active:opacity-70"
					aria-label="Add an exercise to {s.name}"
					onclick={() => openLibrary(i)}
				>
					<Icon name="plus" size="1rem" />
					Add
				</button>
			</div>
			{#if s.items.length}
				<ul class="rounded-3xl bg-surface py-1 shadow-card">
					{#each s.items as item, j (item.step?.id ?? item.choice?.id ?? j)}
						{#if item.step}
							{@const reached = item.step.status !== null}
							{#if j > 0 && s.items[j - 1].choice}
								<li class="px-3.5 pt-2.5 pb-0.5 text-xs font-semibold tracking-[0.06em] text-ink-3 uppercase">Then</li>
							{/if}
							<li class="flex min-h-14 items-center gap-3 px-3.5 py-2.5 {j > 0 && s.items[j - 1].step ? 'shadow-[inset_0_1px_0_var(--line)]' : ''}">
								{#if reached}
									<span class="flex size-7 shrink-0 items-center justify-center rounded-full bg-ink text-ground dark:bg-ink-2">
										<Icon name="check" size="1rem" />
									</span>
								{:else}
									{@render remove(`Remove ${item.step.name}`, () => openRun.removeStep(item.step!.id))}
								{/if}
								<span class="min-w-0 flex-1">
									<span class="block truncate text-[15px] font-bold {reached ? 'text-ink-2' : ''}">{item.step.name}</span>
									<span class="mt-0.5 block truncate text-xs font-semibold text-ink-2">
										{item.step.status === 'skipped' ? 'Skipped' : stepMeta(item.step)}
									</span>
								</span>
							</li>
						{:else}
							{@const c = item.choice}
							<li class="px-3.5 pt-2.5 pb-0.5 text-xs font-semibold tracking-[0.06em] text-ink-3 uppercase">
								{choiceMeta(c)}
							</li>
							{#each c.options as o, k (k)}
								<li class="flex min-h-14 items-center gap-3 px-3.5 py-2.5 {k > 0 ? 'shadow-[inset_0_1px_0_var(--line)]' : ''}">
									{@render remove(`Remove ${o.name}`, () => openRun.removeOption(c.id, k))}
									<span class="min-w-0 flex-1">
										<span class="block truncate text-[15px] font-bold">{o.name}</span>
										<span class="mt-0.5 block truncate text-xs font-semibold text-ink-2">{stepMeta(o)}</span>
									</span>
								</li>
							{/each}
						{/if}
					{/each}
				</ul>
			{/if}
		{/each}
		<div class="mt-5">
			<Button variant="secondary" onclick={() => (naming = true)}>
				<Icon name="plus" />
				Add section
			</Button>
		</div>
	</div>
{:else}
	<div class="flex flex-col px-4 pt-1 pb-[calc(env(safe-area-inset-bottom)+7rem)]">
		<section class="rounded-3xl bg-surface px-5 pt-4 pb-[18px] shadow-card">
			<div class="flex items-start justify-between gap-3">
				<div>
					<p class="text-xl leading-tight font-bold tracking-[-0.01em]">{minutes} min</p>
					<p class="mt-0.5 text-xs font-semibold text-ink-2">{finished} of {steps.length} exercises</p>
				</div>
				<div class="text-right">
					<p class="text-xl leading-tight font-bold tracking-[-0.01em]">
						{#if run.finished_at}
							Finished
						{:else if section >= 0}
							Section {section + 1} of {run.sections.length}
						{:else}
							All done
						{/if}
					</p>
					{#if run.sections.length}
						<p class="mt-0.5 text-xs font-semibold text-ink-2">
							{sectionsDone} done · {run.sections.length - sectionsDone} left
						</p>
					{/if}
				</div>
			</div>
			<div
				class="mt-3.5 flex gap-1.5"
				role="progressbar"
				aria-valuemin={0}
				aria-valuemax={steps.length}
				aria-valuenow={finished}
			>
				{#each run.sections as s, i (i)}
					<span class="h-2 flex-1 overflow-hidden rounded-full {marks[i] === 'done' ? 'bg-ink dark:bg-ink-2' : 'bg-well'}">
						{#if marks[i] === 'now'}
							<span class="block h-full rounded-full bg-live" style="width: {share(s) * 100}%"></span>
						{/if}
					</span>
				{/each}
			</div>
		</section>

		<ol class="mt-1.5">
			{#each run.sections as s, i (i)}
				{@const m = marks[i]}
				{@const open = isOpen(i)}
				<li class="relative">
					{#if i < run.sections.length - 1}
						<span
							aria-hidden="true"
							class="absolute top-[26px] -bottom-[26px] left-[14.75px] border-l-[2.5px] {m === 'done'
								? 'border-ink dark:border-ink-2'
								: 'border-dotted border-ink-3 opacity-70'}"
						></span>
					{/if}
					<button
						type="button"
						class="relative flex w-full items-center gap-3.5 pr-1 text-left {m === 'now' ? 'h-11' : 'h-[52px]'}"
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
							<span class="shrink-0 rounded-[10px] bg-live px-2.5 py-1 text-xs font-bold text-on-live">You are here</span>
						{:else}
							{#if m === 'done'}
								<span class="flex shrink-0 gap-1" aria-hidden="true">
									{#each s.items as it, k (k)}
										<span
											class="size-[7px] rounded-full {it.step?.status === 'skipped'
												? 'border-[1.5px] border-ink-3'
												: 'bg-ink dark:bg-ink-2'}"
										></span>
									{/each}
								</span>
							{/if}
							<span class="text-ink-3 transition-transform {open ? 'rotate-90' : ''}">
								<Icon name="chevron-right" size="1.125rem" />
							</span>
						{/if}
					</button>

					{#if open && s.items.length}
						<ul class="relative mb-1 ml-[46px] rounded-3xl bg-surface p-2.5 pb-2 shadow-card">
							{#each s.items as item, j (item.step?.id ?? item.choice?.id ?? j)}
								<li class="[&:not(:first-child)]:mt-1 [&:not(:first-child)]:shadow-[inset_0_1px_0_var(--line)]">
									{#if item.step}
										{@const sm = mark(item.step)}
										<a href="/run/{run.id}/step/{item.step.id}" class="flex items-center gap-3 px-1 py-2.5">
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
																	? 'bg-ink dark:bg-ink-2'
																	: n === logged
																		? 'bg-live'
																		: 'bg-well'}"
															></span>
														{/each}
														{#if logged < item.step.sets}
															<span class="ml-0.5 text-xs font-bold">Set {logged + 1} next</span>
														{/if}
													</span>
												{/if}
											</span>
											<span class="text-ink-3"><Icon name="chevron-right" size="1.125rem" /></span>
										</a>
									{:else}
										{@const c = item.choice}
										<div id="pick-{c.id}" class="scroll-mt-20 pt-0.5 pb-1">
											<p class="px-1 pt-0.5 pb-2 text-xs font-semibold tracking-[0.06em] text-ink-2 uppercase">
												{c.name} · {choiceMeta(c)}
											</p>
											<div class="grid grid-cols-2 gap-2">
												{#each c.options as o, k (k)}
													{@const on = choosing === c.id && picked.includes(k)}
													{@const thumb = o.media.find((md) => md.thumb_url)?.thumb_url}
													<button
														type="button"
														class="relative overflow-hidden rounded-[18px] text-left {on
															? 'bg-surface shadow-[0_0_0_2.5px_var(--live),var(--elev-sm)]'
															: 'bg-well opacity-90'}"
														aria-pressed={on}
														onclick={() => toggle(c.id, k)}
													>
														{#if thumb}
															<img src={thumb} alt="" class="h-14 w-full object-cover" />
														{/if}
														<span
															class="absolute top-2 right-2 flex size-6 items-center justify-center rounded-full {on
																? 'bg-live text-on-live'
																: thumb
																	? 'border-2 border-white/85 bg-white/30'
																	: 'border-2 border-ink-3'}"
														>
															{#if on}
																<Icon name="check" size="0.875rem" stroke={3} />
															{/if}
														</span>
														<span class="block px-2.5 pt-2 pb-2.5 {thumb ? '' : 'pr-9'}">
															<span class="block text-[15px] leading-tight font-bold">{o.name}</span>
															<span class="mt-0.5 block text-xs font-semibold text-ink-2">{stepMeta(o)}</span>
														</span>
													</button>
												{/each}
											</div>
											{#if choosing === c.id}
												<div class="mt-2.5">
													<Button
														disabled={picked.length < Math.max(1, c.pick)}
														onclick={() => pick(c.id, picked.map((k) => c.options[k]))}
													>
														Use {picked.length === 1 ? 'this' : 'these'}
													</Button>
												</div>
											{/if}
										</div>
									{/if}
								</li>
							{/each}
						</ul>
					{/if}
				</li>
			{/each}
		</ol>

		{#if !run.sections.length}
			<div class="mt-5">
				<Button variant="secondary" onclick={() => openLibrary()}>Add exercise</Button>
			</div>
		{/if}
	</div>
{/if}

<Sheet bind:open={picking} eyebrow={into === undefined ? undefined : run.sections[into]?.name} title="Add exercise">
	<ExercisePicker
		id="add-exercise"
		label="Exercise"
		exercises={library}
		pick={(e) => {
			openRun.addStep(e, into);
			picking = false;
		}}
	/>
</Sheet>

<Sheet bind:open={naming} title="Add section">
	<form onsubmit={addSection}>
		<label class="block text-xs font-semibold text-ink-2" for="section-name">Name</label>
		<input id="section-name" class="mt-1.5 w-full input" maxlength="200" required bind:value={sectionName} />
		<div class="mt-4">
			<Button type="submit" disabled={!sectionName.trim()}>Add section</Button>
		</div>
	</form>
</Sheet>

{#if !editing && !run.finished_at && (stop || steps.length)}
	<div
		class="fixed inset-x-0 bottom-0 z-20 mx-auto flex w-full max-w-[430px] gap-2.5 bg-linear-to-t from-ground from-60% to-transparent px-4 pt-6 pb-[calc(env(safe-area-inset-bottom)+0.75rem)]"
	>
		{#if stop}
			<a
				href="/run/{run.id}/finish"
				class="flex h-14 shrink-0 items-center gap-2 rounded-full bg-surface px-[22px] text-[15px] font-bold text-ink shadow-card active:opacity-80"
			>
				<svg viewBox="0 0 24 24" class="size-[18px]" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M6 21V4M6 4h11l-2 4 2 4H6" /></svg>
				Finish
			</a>
			<div class="min-w-0 flex-1">
				<Button variant="live" href={stopHref(run.id, stop)}>
					<Icon name="play" size="18px" />
					Continue
				</Button>
			</div>
		{:else}
			<div class="flex-1">
				<Button href="/run/{run.id}/finish">Finish session</Button>
			</div>
		{/if}
	</div>
{/if}
