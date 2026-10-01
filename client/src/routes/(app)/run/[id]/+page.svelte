<script lang="ts">
	import { goto } from '$app/navigation';
	import { describe, request } from '$lib/api';
	import Button from '$lib/Button.svelte';
	import FormError from '$lib/FormError.svelte';
	import Icon from '$lib/Icon.svelte';
	import Menu from '$lib/Menu.svelte';
	import NavBar from '$lib/NavBar.svelte';
	import { currentStep, isFinished, secondsSince, setsOf, stepsOf, type RunSection, type RunStep } from '$lib/run';
	import { openRun } from '$lib/runState.svelte';
	import { choiceMeta, stepMeta, type Step } from '$lib/template';
	import type { Exercise } from '$lib/exercise';
	import ExercisePicker from '$lib/ExercisePicker.svelte';
	import SessionDot from '$lib/SessionDot.svelte';

	const run = $derived(openRun.run!);
	const steps = $derived(stepsOf(run));
	const finished = $derived(steps.filter(isFinished).length);
	const current = $derived(currentStep(run));
	const section = $derived(
		current ? run.sections.findIndex((s) => s.items.some((i) => i.step?.id === current.id)) : -1
	);

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

	let library = $state<Exercise[] | null>(null);

	async function openLibrary() {
		error = '';
		try {
			const { exercises } = await request<{ exercises: Exercise[] }>('GET', '/api/v1/exercises');
			library = exercises.filter((e) => !e.retired_at);
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
	const isOpen = (i: number) => chosen[i] ?? i === section;
</script>

<svelte:head><title>{run.name}</title></svelte:head>

<NavBar title={run.name} back={{ href: '/', label: 'Today' }}>
	{#snippet actions()}
		<Menu
			items={[
				{ label: 'Finish session', onclick: () => goto(`/run/${run.id}/finish`) },
				{ label: 'Discard session', danger: true, onclick: discard }
			]}
		/>
	{/snippet}
</NavBar>

<div class="px-4">
	<FormError message={error} />
</div>

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
							<li class="border-line [&:not(:first-child)]:mt-1 [&:not(:first-child)]:border-t">
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
									<div class="pt-0.5 pb-1">
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
															<svg viewBox="0 0 24 24" class="size-3.5" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M5 12.5l4.5 4.5L19 7" /></svg>
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

	<div class="mt-5">
		{#if library}
			<div class="rounded-3xl bg-surface p-4 shadow-card">
				<ExercisePicker
					id="add-exercise"
					exercises={library}
					pick={(e) => {
						openRun.addStep(e);
						library = null;
					}}
				/>
			</div>
		{:else}
			<Button variant="secondary" onclick={openLibrary}>Add exercise</Button>
		{/if}
	</div>
</div>

{#if !run.finished_at && (current || steps.length)}
	<div
		class="fixed inset-x-0 bottom-0 z-20 mx-auto flex w-full max-w-[430px] gap-2.5 bg-linear-to-t from-ground from-60% to-transparent px-4 pt-6 pb-[calc(env(safe-area-inset-bottom)+0.75rem)]"
	>
		{#if current}
			<a
				href="/run/{run.id}/finish"
				class="flex h-14 shrink-0 items-center gap-2 rounded-full bg-surface px-[22px] text-[15px] font-bold text-ink shadow-card active:opacity-80"
			>
				<svg viewBox="0 0 24 24" class="size-[18px]" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M6 21V4M6 4h11l-2 4 2 4H6" /></svg>
				Finish
			</a>
			<div class="min-w-0 flex-1">
				<Button variant="live" href="/run/{run.id}/step/{current.id}">
					<svg viewBox="0 0 24 24" class="size-[18px]" fill="currentColor" aria-hidden="true"><path d="M8 5.5v13l10.5-6.5z" /></svg>
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
