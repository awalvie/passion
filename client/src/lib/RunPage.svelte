<script lang="ts" module>
	import type { MenuItem } from './Menu.svelte';
	import type { RunStep } from './run';
	import type { NextLine } from './template';

	// What the step page hands every player.
	export type PlayerProps = {
		step: RunStep;
		next: NextLine;
		nextHref: string;
		last: boolean;
		menu: MenuItem[];
		error: string;
		leaving: boolean;
		leave: (end: () => void) => Promise<void>;
		skip: () => void;
	};
</script>

<script lang="ts">
	import type { Snippet } from 'svelte';
	import { setSound, soundOn, unlock } from './audio';
	import Icon from './Icon.svelte';
	import JournalSheet from './JournalSheet.svelte';
	import Menu from './Menu.svelte';
	import Notes from './Notes.svelte';
	import RunField, { type RunLook } from './RunField.svelte';
	import { openRun } from './runState.svelte';
	import SaveStatus from './SaveStatus.svelte';
	import Sheet from './Sheet.svelte';
	import type { Step } from './template';
	import { plainText } from './text';

	let {
		step,
		look = 'run',
		next,
		menu,
		band = null,
		error = '',
		reading = $bindable(false),
		done = null,
		skip,
		sounds = false,
		children,
		buttons
	}: {
		step: RunStep;
		look?: RunLook;
		next: NextLine | null;
		menu: MenuItem[];
		band?: number | null;
		error?: string;
		reading?: boolean;
		// Once the step is over, the dock only goes on: to the next stop, or
		// back to the session after the last.
		done?: { href: string; last: boolean } | null;
		skip?: () => void;
		// A player that plays tones shows a switch for them in the bar.
		sounds?: boolean;
		children: Snippet;
		buttons: Snippet;
	} = $props();

	const run = $derived(openRun.run!);
	let sound = $state(soundOn());

	let noting = $state(false);
	let peeking = $state(false);
	let journaling = $state(false);
	let draft = $state('');

	// The step the note sheet opened on, which keeps the note if the page
	// moves on while the sheet is open.
	let noteStep: RunStep | null = null;

	function saveNote() {
		const target = noteStep ?? step;
		const text = draft.trim() || null;
		if (text === (target.run_notes ?? null)) return;
		target.run_notes = text;
		openRun.saveBody();
	}

	// A sheet left open must not follow the page to the next step, and a note
	// typed there belongs to the step it was typed on.
	let shown = '';
	$effect.pre(() => {
		if (step.id === shown) return;
		shown = step.id;
		reading = false;
		peeking = false;
		if (noting) saveNote();
		noting = false;
	});

	function openNote() {
		noteStep = step;
		draft = step.run_notes ?? '';
		noting = true;
	}

	// The name takes two lines at most: it starts at 28 px and steps down to
	// 22 px until it fits.
	function fit(node: HTMLElement, name: string) {
		const size = () => {
			let px = 28;
			node.style.fontSize = `${px}px`;
			while (px > 22 && node.scrollHeight > px * 1.08 * 2 + 2) {
				px -= 1;
				node.style.fontSize = `${px}px`;
			}
		};
		void document.fonts?.ready.then(size);
		size();
		return { update: size };
	}
</script>

{#snippet guide(s: Step)}
	{@const video = s.media?.find((m) => m.url)}
	<div class="flex flex-col gap-4 px-1 text-[15px] leading-[1.4] font-semibold text-ink">
		{#if video?.thumb_url}
			<a
				href={video.url}
				target="_blank"
				rel="noopener noreferrer"
				class="relative flex aspect-video items-center justify-center overflow-hidden rounded-[18px] bg-well shadow-card-sm"
				aria-label="Watch the video"
			>
				<img src={video.thumb_url} alt="" class="absolute inset-0 size-full object-cover" />
				<span class="relative flex size-14 items-center justify-center rounded-full bg-white/90 pl-1 text-on-tint">
					<Icon name="play" size="1.5rem" />
				</span>
			</a>
		{:else if video}
			<a href={video.url} target="_blank" rel="noopener noreferrer" class="flex h-12 items-center justify-center gap-2 rounded-full bg-surface text-[15px] font-bold shadow-card-sm">
				<Icon name="play" size="1rem" />Watch the video
			</a>
		{/if}
		{#if plainText(s.notes ?? '').trim()}
			<Notes text={s.notes ?? ''} outline class="rounded-3xl bg-surface p-[18px] shadow-card-sm" />
		{:else}
			<p class="text-ink-2">No notes for this exercise yet.</p>
		{/if}
	</div>
{/snippet}

<RunField {look} label={step.name} back={{ href: `/run/${run.id}`, label: 'Back to the session' }} {band}>
	{#snippet end()}
		{#if sounds}
			<button
				type="button"
				class="flex size-11 items-center justify-center rounded-full bg-(--pill) text-on-hero"
				aria-label="Timer sounds"
				aria-pressed={sound}
				onclick={() => setSound((sound = !sound))}
			>
				<Icon name={sound ? 'volume-2' : 'volume-x'} size="1.25rem" />
			</button>
		{/if}
		<Menu
			look="bg-(--pill) text-on-hero"
			items={run.finished_at ? menu : [{ label: 'Write in journal', onclick: () => (journaling = true) }, ...menu]}
		/>
	{/snippet}

	<div class="flex flex-col gap-2 px-4 empty:hidden">
		<SaveStatus />
		{#if error}<p class="rounded-2xl bg-surface px-4 py-3 text-[15px] font-semibold text-bad" role="alert">{error}</p>{/if}
	</div>

	<h1 class="mt-3 line-clamp-2 shrink-0 px-5 text-center text-[28px] leading-[1.08] font-extrabold tracking-[-0.02em] break-words text-balance" use:fit={step.name}>
		{step.name}
	</h1>
	<div class="mt-3 flex shrink-0 justify-center gap-2 px-4">
		<button type="button" class="run-tag" onclick={() => (reading = true)}>
			<Icon name="book" size="17px" stroke={2.2} />How-to
		</button>
		<button type="button" class="run-tag" aria-label={step.run_notes ? 'Your note' : 'Write a note'} onclick={openNote}>
			<Icon name="notepad" size="17px" stroke={2.2} />Note
			{#if step.run_notes}<i class="size-2 rounded-full bg-tint" aria-hidden="true"></i>{/if}
		</button>
	</div>

	<div class="flex min-h-0 flex-1 flex-col overflow-y-auto overscroll-contain">
		{@render children()}
	</div>

	<div
		class="shrink-0 rounded-t-[28px] bg-(--dock) px-4 pt-5 pb-[calc(env(safe-area-inset-bottom)+1.25rem)] shadow-[inset_0_1px_0_rgba(255,255,255,0.08)]"
		onpointerdown={unlock}
		role="group"
		aria-label="Exercise"
	>
		{#snippet upNext(n: NextLine)}
			<span class="flex min-w-0 flex-1 flex-col gap-0.5">
				<span class="truncate text-sm font-medium text-(--fg2)">
					<b class="font-[family-name:var(--font-digits)] font-bold tracking-[0.1em]">NEXT</b>{n.detail ? ` · ${n.detail}` : ''}
				</span>
				<span class="truncate text-[17px] leading-[1.2] font-bold">{n.name}</span>
			</span>
		{/snippet}
		{#if next?.step}
			<button type="button" class="flex min-h-11 w-full items-center gap-2.5 px-1 text-left" onclick={() => (peeking = true)}>
				{@render upNext(next)}
				<span class="flex size-8 shrink-0 items-center justify-center rounded-full bg-(--glass) text-(--fg2)" aria-hidden="true">
					<Icon name="chevron-right" size="18px" stroke={2.4} />
				</span>
			</button>
		{:else if next}
			<div class="flex min-h-11 items-center gap-2.5 px-1">{@render upNext(next)}</div>
		{/if}
		<div class="flex gap-2.5 {next ? 'mt-4' : ''}">
			{#if done}
				<a href={done.href} class="run-btn bg-live text-on-live">
					{done.last ? 'Back to session' : 'Next exercise'}<span class="run-icon"><Icon name="arrow-right" size="16px" stroke={2.6} /></span>
				</a>
			{:else}
				{@render buttons()}
				{#if skip}<button type="button" class="run-round" aria-label="Skip" onclick={skip}><Icon name="skip" size="26px" /></button>{/if}
			{/if}
		</div>
	</div>

	<Sheet bind:open={reading} title={step.name} eyebrow="How to">
		{@render guide(step)}
	</Sheet>

	{#if next?.step}
		<Sheet bind:open={peeking} title={next.step.name} eyebrow={next.detail ? `Up next · ${next.detail}` : 'Up next'}>
			{@render guide(next.step)}
		</Sheet>
	{/if}

	<Sheet bind:open={noting} title="Your note" eyebrow={step.name}>
		<textarea
			class="min-h-36 w-full rounded-3xl bg-surface p-4 text-[15px] font-semibold text-ink outline-none placeholder:text-ink-3"
			placeholder="How did it go?"
			aria-label="Note"
			bind:value={draft}
			onchange={saveNote}
			onblur={saveNote}
		></textarea>
	</Sheet>

	<JournalSheet bind:open={journaling} />
</RunField>
