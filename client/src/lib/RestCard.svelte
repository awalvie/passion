<script lang="ts">
	import { formatClock, readTimers, writeTimers } from './timerStore';

	let { runId, seconds }: { runId: string; seconds: number | null } = $props();

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

	function save(next: number | null) {
		endsAt = next;
		now = Date.now();
		writeTimers(runId, { ...readTimers(runId), rest: next === null ? null : { endsAt: next } });
	}

	export function start() {
		if (seconds) save(Date.now() + seconds * 1000);
	}
</script>

{#if endsAt !== null && left > 0}
	<section class="flex items-center gap-3 rounded-2xl bg-rest p-4 text-on-tint shadow-sm" aria-live="polite">
		<div class="flex-1">
			<p class="text-sm opacity-90">Rest</p>
			<p class="text-3xl font-semibold tabular-nums">{formatClock(left)}</p>
		</div>
		<button type="button" class="h-11 rounded-xl bg-white/20 px-4 text-base font-semibold" onclick={() => save(endsAt! + 30_000)}>
			+30 s
		</button>
		<button type="button" class="h-11 rounded-xl bg-white/20 px-4 text-base font-semibold" onclick={() => save(null)}>
			Skip
		</button>
	</section>
{:else if seconds}
	<button type="button" class="h-11 rounded-xl bg-surface text-base font-semibold text-rest shadow-sm" onclick={start}>
		Start rest · {formatClock(seconds)}
	</button>
{/if}
