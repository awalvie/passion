<script lang="ts">
	// A short note above the tab bar, with one action. It closes by itself
	// after duration milliseconds.
	let {
		message = $bindable(''),
		action,
		onaction,
		duration = 6000
	}: { message?: string; action?: string; onaction?: () => void; duration?: number } = $props();

	$effect(() => {
		if (!message) return;
		const t = setTimeout(() => (message = ''), duration);
		return () => clearTimeout(t);
	});
</script>

{#if message}
	<div
		class="fixed inset-x-0 bottom-[var(--above-bar)] z-40 mx-auto flex min-h-14 w-[calc(100%-2rem)] max-w-[398px] items-center gap-3 rounded-full bg-hero py-2 pr-2 pl-5 text-on-hero shadow-card"
		role="status"
	>
		<span class="min-w-0 flex-1 truncate text-[15px] font-bold">{message}</span>
		{#if action}
			<button
				type="button"
				class="h-11 shrink-0 rounded-full bg-tint px-4 text-[15px] font-bold text-on-tint"
				onclick={() => {
					message = '';
					onaction?.();
				}}
			>
				{action}
			</button>
		{/if}
	</div>
{/if}
