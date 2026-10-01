<script lang="ts">
	import { page } from '$app/state';
	import { formatDuration } from './exercise';
	import Icon, { type IconName } from './Icon.svelte';
	import { secondsSince, type RunSummary } from './run';
	import { readTimers } from './timerStore';

	// live is the open session, shown as a strip on top of the tabs on every
	// tab page.
	let { live }: { live: RunSummary | null } = $props();

	let now = $state(Date.now());
	$effect(() => {
		if (!live) return;
		const tick = setInterval(() => (now = Date.now()), 1000);
		return () => clearInterval(tick);
	});
	const minutes = $derived(live ? Math.floor(secondsSince(live.started_at, now) / 60) : 0);
	// A rest shows only on the phone that started it, which holds its end time.
	const restLeft = $derived(live ? ((readTimers(live.id).rest?.endsAt ?? 0) - now) / 1000 : 0);

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
	{#if live}
		<a
			href="/run/{live.id}"
			class="flex h-11 items-center gap-2.5 rounded-t-[28px] pr-4 pl-6 {restLeft > 0 ? 'bg-rest-card text-rest-digit' : 'bg-live text-on-live'}"
		>
			{#if restLeft > 0}
				<span class="shrink-0 text-xs font-extrabold tracking-[0.06em] uppercase">Rest</span>
			{:else}
				<span class="size-2 shrink-0 rounded-full bg-on-live motion-safe:animate-pulse"></span>
			{/if}
			<span class="min-w-0 truncate text-[15px] font-bold">{live.name}</span>
			<span class="shrink-0 text-xs font-semibold">· {restLeft > 0 ? `under ${Math.ceil(restLeft / 60)} min` : formatDuration(Math.max(1, minutes) * 60)}</span>
			<span class="ml-auto shrink-0"><Icon name="chevron-right" size="1.125rem" /></span>
		</a>
	{/if}
	<div class="flex h-16 items-center justify-around px-3 pt-1">
		{#each tabs as t (t.href)}
			{@const active = t.match(page.url.pathname)}
			<a
				href={t.href}
				class="flex w-20 flex-col items-center gap-0.5 text-xs font-bold {active ? 'text-ink' : 'text-ink-2'}"
				aria-current={active ? 'page' : undefined}
			>
				<span class="flex h-8 w-14 items-center justify-center rounded-full {active ? 'bg-tint text-on-tint' : ''}">
					<Icon name={t.icon} size="1.375rem" />
				</span>
				{t.label}
			</a>
		{/each}
	</div>
</nav>
