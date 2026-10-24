<script lang="ts" module>
	export type RunLook = 'run' | 'prep' | 'hang' | 'rest';
</script>

<script lang="ts">
	import type { Snippet } from 'svelte';
	import Icon from './Icon.svelte';
	import { secondsSince } from './run';
	import { openRun } from './runState.svelte';
	import { runRings } from './topo';
	import { sessionClock } from './timerStore';

	let {
		look = 'run',
		label,
		back,
		band = null,
		end,
		children
	}: {
		look?: RunLook;
		label: string;
		// swipe lets a swipe in from the left edge follow the link, as NavBar's does.
		back?: { href: string; label: string; swipe?: boolean };
		band?: number | null;
		end: Snippet;
		children: Snippet;
	} = $props();

	const run = $derived(openRun.run!);
	// A finished run being edited shows the time it took, not a running clock.
	const done = $derived(run.finished_at !== null);

	// Text, glass and contour colours for each field. The field keeps one look
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
</script>

<section
	class="fixed inset-0 z-40 overflow-hidden text-(--fg) [color-scheme:dark]"
	style="--pill:rgba(10,20,15,.45);{looks[look]};background:var(--field)"
	aria-label={label}
>
	{#if band !== null}
		<div class="absolute inset-x-0 bottom-0 bg-black/10 shadow-[0_-2px_0_rgba(0,0,0,0.08)]" style="height: {band * 100}%" aria-hidden="true"></div>
	{/if}
	<div class="pointer-events-none absolute inset-x-0 top-[calc(env(safe-area-inset-top)-46px)] mx-auto h-[844px] w-[390px]" aria-hidden="true">
		<svg viewBox={runRings.viewBox} class="absolute inset-0 size-full" style="color: rgb(var(--rings) / 0.07)">
			<path d={runRings.d} fill="none" stroke="currentColor" stroke-width="1.2" />
		</svg>
		{#if !done}
			<svg viewBox={runRings.viewBox} class="pulse absolute inset-0 size-full" style="color: rgb(var(--rings) / 0.35)">
				<path d={runRings.d} fill="none" stroke="currentColor" stroke-width="1.2" />
			</svg>
		{/if}
	</div>

	<div class="run-tokens relative mx-auto flex h-full w-full max-w-[430px] flex-col pt-[env(safe-area-inset-top)]">
		<!-- Three columns keep the clock centred however many buttons sit at the end. -->
		<div class="grid h-14 shrink-0 grid-cols-[1fr_auto_1fr] items-center px-4">
			{#if back}
				<a href={back.href} data-back={back.swipe || undefined} class="flex size-11 items-center justify-center rounded-full bg-(--pill) text-on-hero" aria-label={back.label}>
					<Icon name="chevron-left" size="22px" stroke={2.6} />
				</a>
			{:else}
				<span class="size-11"></span>
			{/if}
			{#if done}
				<span class="flex h-9 items-center rounded-full bg-(--pill) px-3 font-[family-name:var(--font-digits)] text-lg font-bold text-on-hero" aria-label="Session time">
					{sessionClock(run.elapsed_seconds ?? secondsSince(run.started_at, Date.parse(run.finished_at!)))}
				</span>
			{:else}
				<span
					class="flex h-9 items-center gap-2 rounded-full border-[1.5px] border-live bg-(--pill) px-3 font-[family-name:var(--font-digits)] text-lg font-bold text-on-hero [animation:run-beat_4s_ease-out_infinite]"
					aria-label="Session time"
				>
					<i class="size-[9px] rounded-full bg-live"></i>
					{sessionClock(secondsSince(run.started_at, now))}
				</span>
			{/if}
			<div class="flex justify-end gap-2">{@render end()}</div>
		</div>
		{@render children()}
	</div>
</section>

<style>
	.pulse {
		mask-image: radial-gradient(circle at 195px 74px, transparent calc(var(--wave) - 46px), #000 var(--wave), transparent calc(var(--wave) + 46px));
		animation: run-wave 4s linear infinite;
	}
	/* The app's own parts inside the field read dark on it. */
	.run-tokens {
		--ground: #12231a;
		--surface: #1d3527;
		--well: #284637;
		--line: rgba(242, 246, 234, 0.08);
		--ink: #f2f6ea;
		--ink-2: rgba(242, 246, 234, 0.7);
		--ink-3: rgba(242, 246, 234, 0.45);
		--chart: #6ad9a0;
		--bad: #f07a63;
		--elev: inset 0 1px 0 rgba(255, 255, 255, 0.05), 0 14px 28px -10px rgba(0, 0, 0, 0.5);
		--elev-sm: inset 0 1px 0 rgba(255, 255, 255, 0.05), 0 6px 14px -4px rgba(0, 0, 0, 0.4);
		--elev-tint: none;
	}
</style>
