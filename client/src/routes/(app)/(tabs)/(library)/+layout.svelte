<script lang="ts">
	import { page } from '$app/state';
	import Icon from '$lib/Icon.svelte';

	let { children } = $props();

	const parts = [
		{ href: '/templates', label: 'Sessions', add: 'New session template' },
		{ href: '/exercises', label: 'Exercises', add: 'New exercise' }
	];

	const list = $derived(parts.find((p) => page.url.pathname === p.href));
	// A session or exercise page draws its own nav bar with a back pill.
	const detail = $derived(page.route.id?.endsWith('/[id]') ?? false);
</script>

{#if detail}
	{@render children()}
{:else}
	<div class="flex flex-col gap-3.5 px-4 pt-[calc(env(safe-area-inset-top)+0.75rem)]">
		{#if list}
			<header class="flex items-start justify-between">
				<h1 class="text-[32px] leading-[1.1] font-extrabold tracking-[-0.02em]">Library</h1>
				<a
					href="{list.href}/new"
					class="flex size-10 shrink-0 items-center justify-center rounded-full bg-surface text-ink shadow-card-sm"
					aria-label={list.add}
					title={list.add}
				>
					<Icon name="plus" size="1.25rem" />
				</a>
			</header>
		{/if}
		<nav class="grid grid-cols-2 gap-1 rounded-full bg-well p-1 dark:bg-surface">
			{#each parts as p (p.href)}
				{@const active = page.url.pathname.startsWith(p.href)}
				<a
					href={p.href}
					class="flex h-10 items-center justify-center rounded-full text-[15px] font-bold {active ? 'bg-tint text-on-tint shadow-card-sm' : 'text-ink-2'}"
					aria-current={active ? 'page' : undefined}
				>
					{p.label}
				</a>
			{/each}
		</nav>
		<main>
			{@render children()}
		</main>
	</div>
{/if}
