<script lang="ts" module>
	export type RunLook = 'run' | 'prep' | 'hang' | 'rest';
</script>

<script lang="ts">
	import type { Snippet } from 'svelte';
	import { unlock } from './audio';
	import Icon, { type IconName } from './Icon.svelte';
	import Menu, { type MenuItem } from './Menu.svelte';
	import Notes from './Notes.svelte';
	import { secondsSince, type RunStep } from './run';
	import { openRun } from './runState.svelte';
	import SaveStatus from './SaveStatus.svelte';
	import Sheet from './Sheet.svelte';
	import { plainText } from './text';
	import { runRings } from './topo';
	import { sessionClock } from './timerStore';

	let {
		step,
		look = 'run',
		icon,
		status,
		next,
		menu,
		band = null,
		error = '',
		reading = $bindable(false),
		children,
		buttons
	}: {
		step: RunStep;
		look?: RunLook;
		icon: IconName;
		status: string;
		next: string | null;
		menu: MenuItem[];
		band?: number | null;
		error?: string;
		reading?: boolean;
		children: Snippet;
		buttons: Snippet;
	} = $props();

	const run = $derived(openRun.run!);

	// Text, glass and contour colours for each field. The page keeps one look
	// in both themes; the app's tokens are set dark inside it, so a stepper,
	// a sheet or the menu reads as part of it.
	const looks: Record<RunLook, string> = {
		run: '--field:var(--run);--fg:#f2f6ea;--fg2:rgba(242,246,234,.7);--glass:rgba(255,255,255,.1);--accent:var(--fg);--rings:198 240 91;--dock:color-mix(in srgb,var(--field) 72%,black)',
		prep: '--field:var(--prep);--fg:#f6f9f4;--fg2:rgba(246,249,244,.78);--glass:rgba(255,255,255,.16);--accent:var(--fg);--rings:232 239 255;--dock:color-mix(in srgb,var(--field) 78%,black)',
		hang: '--field:var(--hang);--fg:#101812;--fg2:rgba(16,24,18,.74);--glass:rgba(16,24,18,.12);--accent:var(--fg);--rings:16 24 18;--dock:color-mix(in srgb,var(--field) 87%,black);--pill:rgba(10,20,15,.72)',
		rest: '--field:var(--rest-field);--fg:#eefaf6;--fg2:rgba(238,250,246,.74);--glass:rgba(255,255,255,.12);--accent:var(--rest-glyph);--rings:164 242 223;--dock:color-mix(in srgb,var(--field) 72%,black)'
	};

	let now = $state(Date.now());
	$effect(() => {
		const tick = setInterval(() => (now = Date.now()), 1000);
		return () => clearInterval(tick);
	});

	const howTo = $derived(plainText(step.notes ?? '').trim());
	const video = $derived(step.media?.find((m) => m.url));

	let noting = $state(false);
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
		if (noting) saveNote();
		noting = false;
	});

	function openNote() {
		noteStep = step;
		draft = step.run_notes ?? '';
		noting = true;
	}

	// The name takes two lines at most: it starts at 32 px and steps down to
	// 24 px until it fits.
	function fit(node: HTMLElement, name: string) {
		const size = () => {
			let px = 32;
			node.style.fontSize = `${px}px`;
			while (px > 24 && node.scrollHeight > px * 1.05 * 2 + 2) {
				px -= 1;
				node.style.fontSize = `${px}px`;
			}
		};
		void document.fonts?.ready.then(size);
		size();
		return { update: size };
	}
</script>

<section
	class="fixed inset-0 z-40 overflow-hidden text-(--fg) [color-scheme:dark]"
	style="--pill:rgba(10,20,15,.45);{looks[look]};background:var(--field)"
	aria-label={step.name}
>
	{#if band !== null}
		<div class="absolute inset-x-0 bottom-0 bg-black/10 shadow-[0_-2px_0_rgba(0,0,0,0.08)]" style="height: {band * 100}%" aria-hidden="true"></div>
	{/if}
	<div class="pointer-events-none absolute inset-x-0 top-[calc(env(safe-area-inset-top)-46px)] mx-auto h-[844px] w-[390px]" aria-hidden="true">
		<svg viewBox={runRings.viewBox} class="absolute inset-0 size-full" style="color: rgb(var(--rings) / 0.07)">
			<path d={runRings.d} fill="none" stroke="currentColor" stroke-width="1.2" />
		</svg>
		<svg viewBox={runRings.viewBox} class="pulse absolute inset-0 size-full" style="color: rgb(var(--rings) / 0.35)">
			<path d={runRings.d} fill="none" stroke="currentColor" stroke-width="1.2" />
		</svg>
	</div>

	<div class="run-tokens relative mx-auto flex h-full w-full max-w-[430px] flex-col pt-[env(safe-area-inset-top)]">
		<div class="flex h-14 shrink-0 items-center justify-between px-4">
			<a
				href="/run/{run.id}"
				class="flex size-11 items-center justify-center rounded-full bg-(--pill) text-on-hero"
				aria-label="Back to the session"
			>
				<Icon name="chevron-left" size="22px" stroke={2.6} />
			</a>
			<span
				class="flex h-9 items-center gap-2 rounded-full border-[1.5px] border-live bg-(--pill) px-3 font-[family-name:var(--font-digits)] text-lg font-bold text-on-hero [animation:run-beat_4s_ease-out_infinite]"
				aria-label="Session time"
			>
				<i class="size-[9px] rounded-full bg-live"></i>
				{sessionClock(secondsSince(run.started_at, now))}
			</span>
			<Menu look="bg-(--pill) text-on-hero" items={menu} />
		</div>

		<div class="flex flex-col gap-2 px-4 empty:hidden">
			<SaveStatus />
			{#if error}<p class="rounded-2xl bg-surface px-4 py-3 text-[15px] font-semibold text-bad" role="alert">{error}</p>{/if}
		</div>

		<div class="mt-3 flex h-11 shrink-0 items-center gap-2.5 pr-4 pl-5">
			<span class="flex size-9 shrink-0 items-center justify-center rounded-full bg-(--glass) text-(--accent)">
				<Icon name={icon} size="20px" stroke={2.6} />
			</span>
			<p class="min-w-0 flex-1 truncate font-[family-name:var(--font-digits)] text-[22px] leading-none font-bold tracking-[0.06em] text-(--accent) uppercase">
				{status}
			</p>
			<button type="button" class="flex size-11 shrink-0 items-center justify-center rounded-full bg-(--glass)" aria-label="How to" onclick={() => (reading = true)}>
				<Icon name="book" size="20px" stroke={2.2} />
			</button>
			<button
				type="button"
				class="relative flex size-11 shrink-0 items-center justify-center rounded-full bg-(--glass)"
				aria-label={step.run_notes ? 'Your note' : 'Write a note'}
				onclick={openNote}
			>
				<Icon name="notepad" size="20px" stroke={2.2} />
				{#if step.run_notes}
					<i class="absolute top-px right-px size-[11px] rounded-full bg-tint shadow-[0_0_0_3px_var(--field)]"></i>
				{/if}
			</button>
		</div>
		<h1 class="mt-2 line-clamp-2 shrink-0 px-5 text-[32px] leading-[1.05] font-extrabold tracking-[-0.02em] break-words text-balance" use:fit={step.name}>
			{step.name}
		</h1>

		<div class="flex min-h-0 flex-1 flex-col overflow-y-auto overscroll-contain">
			{@render children()}
		</div>

		<div
			class="shrink-0 rounded-t-[28px] bg-(--dock) px-4 pt-5 pb-[calc(env(safe-area-inset-bottom)+1.25rem)] shadow-[inset_0_1px_0_rgba(255,255,255,0.08)]"
			onpointerdown={unlock}
			role="group"
			aria-label="Exercise"
		>
			{#if next}
				<p class="px-1 font-[family-name:var(--font-digits)] text-[17px] leading-none font-bold tracking-[0.08em] text-(--fg2)">NEXT</p>
				<p class="mt-1 truncate px-1 text-[22px] leading-tight font-extrabold tracking-[-0.01em]">{next}</p>
			{/if}
			<div class="flex gap-2.5 {next ? 'mt-5' : ''}">
				{@render buttons()}
			</div>
		</div>

		<Sheet bind:open={reading} title={step.name} eyebrow="How to">
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
				{#if howTo}
					<Notes text={step.notes ?? ''} outline class="rounded-[22px] bg-surface p-5 shadow-card-sm" />
				{:else}
					<p class="text-ink-2">No notes for this exercise yet.</p>
				{/if}
			</div>
		</Sheet>

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
	</div>
</section>

<style>
	.pulse {
		mask-image: radial-gradient(circle at 195px 74px, transparent calc(var(--wave) - 46px), #000 var(--wave), transparent calc(var(--wave) + 46px));
		animation: run-wave 4s linear infinite;
	}
	/* The app's own parts inside the page read dark on the field. */
	.run-tokens {
		--ground: #12231a;
		--surface: #1d3527;
		--well: #284637;
		--inset: #1a3024;
		--line: rgba(242, 246, 234, 0.08);
		--ink: #f2f6ea;
		--ink-2: rgba(242, 246, 234, 0.7);
		--ink-3: rgba(242, 246, 234, 0.45);
		--link: #f2f6ea;
		--bad: #f07a63;
		--elev: inset 0 1px 0 rgba(255, 255, 255, 0.05), 0 14px 28px -10px rgba(0, 0, 0, 0.5);
		--elev-sm: inset 0 1px 0 rgba(255, 255, 255, 0.05), 0 6px 14px -4px rgba(0, 0, 0, 0.4);
	}
</style>
