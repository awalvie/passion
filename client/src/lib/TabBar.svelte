<script lang="ts">
	import { page } from '$app/state';
	import Icon, { type IconName } from './Icon.svelte';

	let { live = false }: { live?: boolean } = $props();

	const tabs: { href: string; label: string; icon: IconName; match: (path: string) => boolean }[] = [
		{ href: '/', label: 'Today', icon: 'peak', match: (p) => p === '/' },
		{ href: '/plan', label: 'Plan', icon: 'calendar', match: (p) => p.startsWith('/plan') },
		{
			href: '/templates',
			label: 'Library',
			icon: 'stack',
			match: (p) => p.startsWith('/templates') || p.startsWith('/exercises')
		},
		{ href: '/history', label: 'History', icon: 'clock', match: (p) => p.startsWith('/history') }
	];
</script>

<nav
	class="fixed inset-x-0 bottom-0 z-30 mx-auto w-full max-w-[430px] rounded-t-[28px] bg-[var(--bar)] pb-[env(safe-area-inset-bottom)] shadow-[0_-10px_30px_-12px_rgba(21,32,26,0.14)] dark:shadow-[0_-10px_30px_-10px_rgba(0,0,0,0.6),inset_0_1px_0_rgba(255,255,255,0.05)] backdrop-blur-xl"
	aria-label="Main"
>
	<div class="flex h-16 items-center justify-around px-3 pt-1">
		{#each tabs as t (t.href)}
			{@const active = t.match(page.url.pathname)}
			<a
				href={t.href}
				class="flex w-20 flex-col items-center gap-0.5 text-xs font-bold {active ? 'text-ink' : 'text-ink-2'}"
				aria-current={active ? 'page' : undefined}
			>
				<span class="relative flex h-8 w-14 items-center justify-center rounded-full {active ? 'bg-tint text-on-tint' : ''}">
					<Icon name={t.icon} size="1.375rem" />
					{#if live && t.href === '/'}
						<span class="absolute top-0.5 right-2 size-2.5 rounded-full bg-live ring-2 ring-[var(--bar)]"></span>
					{/if}
				</span>
				{t.label}
			</a>
		{/each}
	</div>
</nav>
