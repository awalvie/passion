<script lang="ts" module>
	// Where each page was scrolled to, and the top page each tab was last on,
	// so a tab opens where you left it.
	const scrolled = new Map<string, number>();
	const left = new Map<number, string>();
</script>

<script lang="ts">
	import { beforeNavigate, goto, preloadData } from '$app/navigation';
	import { navigating, page } from '$app/state';
	import { formatDuration } from './exercise';
	import Icon from './Icon.svelte';
	import { secondsSince, type RunSummary } from './run';
	import { tabOf, tabs, under } from './tabs';
	import { readTimers } from './timerStore';

	// live is the open session, shown as a strip on top of the tabs.
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

	// The pill moves on the tap, before the next page has loaded.
	const active = $derived(tabOf(navigating.to?.url.pathname ?? page.url.pathname));

	// The other tabs load once the app has settled, so the first tap on one
	// shows it at once.
	$effect(() => {
		const t = setTimeout(() => tabs.forEach((tab) => preloadData(tab.href).catch(() => {})), 1500);
		return () => clearTimeout(t);
	});

	beforeNavigate(({ from }) => {
		if (!from) return;
		const at = from.url.pathname + from.url.search;
		scrolled.set(at, scrollY);
		const i = tabOf(from.url.pathname);
		if (i >= 0 && tabs[i].roots.includes(from.url.pathname)) left.set(i, at);
	});

	// A tap on another tab opens it where you left it. A tap on the open tab
	// goes up to its top page, or to the top of that page.
	async function open(e: MouseEvent, i: number) {
		if (e.metaKey || e.ctrlKey || e.shiftKey) return;
		e.preventDefault();
		const t = tabs[i];
		const here = page.url.pathname;
		if (i !== active) {
			const to = left.get(i) ?? t.href;
			await goto(to, { noScroll: true });
			scrollTo(0, scrolled.get(to) ?? 0);
		} else if (t.roots.includes(here)) {
			scrollTo({ top: 0, behavior: matchMedia('(prefers-reduced-motion: reduce)').matches ? 'instant' : 'smooth' });
		} else {
			await goto(t.roots.find((r) => under(here, r)) ?? t.href);
		}
	}
</script>

<nav
	class="fixed inset-x-0 bottom-0 z-30 mx-auto w-full max-w-[430px] rounded-t-[28px] bg-[var(--bar)] pb-[env(safe-area-inset-bottom)] shadow-[0_-10px_30px_-12px_rgba(21,32,26,0.14)] dark:shadow-[0_-10px_30px_-10px_rgba(0,0,0,0.6),inset_0_1px_0_rgba(255,255,255,0.05)] backdrop-blur-xl"
	style:view-transition-name="tabbar"
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
	<div class="relative grid h-16 grid-cols-4 items-start px-3 pt-2">
		{#if active >= 0}
			<span
				class="pointer-events-none absolute top-2 left-3 flex w-[calc((100%-1.5rem)/4)] justify-center transition-transform duration-300 ease-[cubic-bezier(0.2,0.8,0.2,1)] motion-reduce:transition-none"
				style:transform="translateX({active * 100}%)"
				aria-hidden="true"
			>
				<span class="h-8 w-14 rounded-full bg-tint"></span>
			</span>
		{/if}
		{#each tabs as t, i (t.href)}
			<a
				href={t.href}
				class="relative flex flex-col items-center gap-0.5 text-xs font-bold {i === active ? 'text-ink' : 'text-ink-2'}"
				aria-current={i === active ? 'page' : undefined}
				onclick={(e) => open(e, i)}
			>
				<span class="flex h-8 w-14 items-center justify-center rounded-full transition-colors duration-300 {i === active ? 'text-on-tint' : ''}">
					<Icon name={t.icon} size="1.375rem" />
				</span>
				{t.label}
			</a>
		{/each}
	</div>
</nav>
