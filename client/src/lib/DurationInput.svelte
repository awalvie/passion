<script lang="ts">
	import { untrack } from 'svelte';

	let {
		seconds = $bindable(),
		id,
		small = false,
		hours = true
	}: { seconds: number | null; id: string; small?: boolean; hours?: boolean } = $props();

	// The three boxes keep what was typed, so 90 minutes stays 90 minutes until
	// the page reloads, rather than jumping to 1 hour 30.
	const start = seconds;
	const split = untrack(() => hours);
	let h = $state(start == null || !split ? null : Math.floor(start / 3600));
	let m = $state(start == null ? null : Math.floor((split ? start % 3600 : start) / 60));
	let s = $state(start == null ? null : start % 60);
	$effect(() => {
		seconds = h == null && m == null && s == null ? null : (h ?? 0) * 3600 + (m ?? 0) * 60 + (s ?? 0);
	});

	const input = $derived(`mt-1.5 w-full input text-center${small ? ' min-h-11 py-2' : ''}`);
</script>

<div class="grid {hours ? 'grid-cols-3' : 'grid-cols-2'} gap-2.5">
	{#if hours}
		<div>
			<label class="block text-xs font-semibold text-ink-2" for="{id}-hours">Hours</label>
			<input id="{id}-hours" type="number" min="0" step="1" class={input} bind:value={h} />
		</div>
	{/if}
	<div>
		<label class="block text-xs font-semibold text-ink-2" for="{id}-minutes">Minutes</label>
		<input id="{id}-minutes" type="number" min="0" step="1" class={input} bind:value={m} />
	</div>
	<div>
		<label class="block text-xs font-semibold text-ink-2" for="{id}-seconds">Seconds</label>
		<input id="{id}-seconds" type="number" min="0" step="1" class={input} bind:value={s} />
	</div>
</div>
