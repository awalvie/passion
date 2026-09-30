<script lang="ts">
	import { page } from '$app/state';
	import Icon, { type IconName } from './Icon.svelte';

	const tabs: { href: string; label: string; icon: IconName; match: (path: string) => boolean }[] = [
		{ href: '/', label: 'Today', icon: 'sun', match: (p) => p === '/' },
		{
			href: '/templates',
			label: 'Library',
			icon: 'book-marked',
			match: (p) => p.startsWith('/templates') || p.startsWith('/exercises')
		},
		{ href: '/history', label: 'History', icon: 'history', match: (p) => p.startsWith('/history') }
	];
</script>

<nav
	class="fixed inset-x-0 bottom-0 z-30 mx-auto w-full max-w-[430px] bg-surface/95 pb-[env(safe-area-inset-bottom)] shadow-[0_-1px_0_var(--line)] backdrop-blur-md"
	aria-label="Main"
>
	<div class="flex h-14">
		{#each tabs as t (t.href)}
			{@const active = t.match(page.url.pathname)}
			<a
				href={t.href}
				class="flex flex-1 flex-col items-center justify-center gap-0.5 text-xs {active ? 'text-tint' : 'text-ink-2'}"
				aria-current={active ? 'page' : undefined}
			>
				<Icon name={t.icon} size="1.5rem" />
				{t.label}
			</a>
		{/each}
	</div>
</nav>
