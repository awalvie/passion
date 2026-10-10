<script lang="ts">
	import { tone, unlock } from './audio';
	import Icon from './Icon.svelte';
	import { formatClock, readTimers, writeTimers } from './timerStore';

	// offer shows the button that starts a rest by hand.
	let { runId, seconds, offer }: { runId: string; seconds: number | null; offer: boolean } = $props();

	let endsAt = $state<number | null>(null);
	$effect.pre(() => {
		endsAt = readTimers(runId).rest?.endsAt ?? null;
	});
	let now = $state(Date.now());
	const left = $derived(endsAt === null ? 0 : (endsAt - now) / 1000);

	$effect(() => {
		if (endsAt === null) return;
		const tick = setInterval(() => (now = Date.now()), 250);
		return () => clearInterval(tick);
	});

	// What +30 s added, so the "of" total grows with it. After a reload the
	// time left is the floor.
	let added = $state(0);
	const total = $derived(Math.max((seconds ?? 0) + added, left));

	function save(next: number | null) {
		endsAt = next;
		now = Date.now();
		writeTimers(runId, { ...readTimers(runId), rest: next === null ? null : { endsAt: next } });
	}

	export function start() {
		unlock();
		added = 0;
		if (seconds) save(Date.now() + seconds * 1000);
	}

	// Two tones and a buzz as the rest runs out, once.
	let rang: number | null = null;
	$effect(() => {
		if (endsAt === null || left > 0 || rang === endsAt) return;
		rang = endsAt;
		if (now - endsAt < 2000) {
			tone(784, 150);
			setTimeout(() => tone(1046, 250), 200);
			navigator.vibrate?.([150, 50, 250]);
		}
	});
	const announce = $derived(endsAt === null ? '' : left > 0 ? 'Rest started' : 'Rest over');
</script>

<p class="sr-only" aria-live="polite">{announce}</p>
{#if endsAt !== null && left > 0}
	<section class="grid grid-cols-[1fr_auto] items-center rounded-3xl bg-rest-field pt-3 pr-3.5 pb-3.5 pl-[18px] text-[#eefaf6]">
		<div>
			<p class="flex items-center gap-2 text-xs font-bold tracking-[0.06em] text-rest-glyph uppercase">
				<i class="size-2.5 rounded-full border-[2.5px] border-current"></i>Rest
			</p>
			<p class="mt-0.5 text-[32px] leading-[1.1] font-extrabold tracking-tight">
				{formatClock(left)}<small class="ml-1.5 text-[15px] font-semibold tracking-normal opacity-75">of {formatClock(total)}</small>
			</p>
		</div>
		<div class="flex gap-2">
			<button
				type="button"
				class="h-11 rounded-full bg-white/12 px-4 text-[15px] font-bold"
				onclick={() => {
					added += 30;
					save(endsAt! + 30_000);
				}}
			>
				+30 s
			</button>
			<button
				type="button"
				class="flex h-11 items-center gap-1.5 rounded-full bg-white/12 px-4 text-[15px] font-bold"
				onclick={() => save(null)}
			>
				Skip
				<Icon name="skip" size="14px" />
			</button>
		</div>
		<div class="col-span-2 mt-2.5 h-1.5 overflow-hidden rounded-full bg-white/15">
			<i class="block h-full rounded-full bg-rest-glyph" style="width: {(1 - left / total) * 100}%"></i>
		</div>
	</section>
{:else if seconds && offer}
	<button
		type="button"
		class="flex h-12 items-center justify-center gap-2 rounded-full bg-rest-field text-[15px] font-bold text-rest-glyph"
		onclick={start}
	>
		<i class="size-2.5 rounded-full border-[2.5px] border-current"></i>Start rest · {formatClock(seconds)}
	</button>
{/if}
