<script lang="ts">
	import { page } from '$app/state';

	let { children } = $props();

	const parts = [
		{ href: '/templates', label: 'Sessions' },
		{ href: '/exercises', label: 'Exercises' }
	];
</script>

<div class="flex flex-col gap-4 px-4 pt-[calc(env(safe-area-inset-top)+0.75rem)]">
	<div class="grid grid-cols-2 gap-1 rounded-xl bg-surface p-1 shadow-sm">
		{#each parts as p (p.href)}
			{@const active = page.url.pathname.startsWith(p.href)}
			<a
				href={p.href}
				class="flex h-9 items-center justify-center rounded-lg text-base {active ? 'bg-tint font-semibold text-on-tint' : 'text-ink'}"
				aria-current={active ? 'page' : undefined}
			>
				{p.label}
			</a>
		{/each}
	</div>
	<main>
		{@render children()}
	</main>
</div>
