<script lang="ts" module>
	// Where each page was scrolled to, and the top page each tab was last on,
	// so a tab opens where you left it.
	const scrolled = new Map<string, number>();
	const left = new Map<number, string>();
</script>

<script lang="ts">
	import { beforeNavigate, goto, preloadData } from '$app/navigation';
	import { navigating, page } from '$app/state';
	import Icon from './Icon.svelte';
	import { currentStep, isFinished, setsOf, secondsSince, stepsOf, type RunSummary } from './run';
	import { openRun, storedRun } from './runState.svelte';
	import { tabOf, tabs, under } from './tabs';
	import { formatClock, readTimers, sessionClock } from './timerStore';

	// live is the open session, shown as a mini player above the tabs.
	let { live }: { live: RunSummary | null } = $props();

	let now = $state(Date.now());
	$effect(() => {
		if (!live) return;
		const tick = setInterval(() => (now = Date.now()), 1000);
		return () => clearInterval(tick);
	});
	// A rest shows only on the phone that started it, which holds its end time.
	const restLeft = $derived(live ? ((readTimers(live.id).rest?.endsAt ?? 0) - now) / 1000 : 0);

	// Only a phone that holds the run knows the step.
	const run = $derived(live ? (openRun.run?.id === live.id ? openRun.run : storedRun(live.id)) : null);
	const step = $derived.by(() => {
		if (!run) return undefined;
		const here = stepsOf(run).find((s) => s.id === openRun.here);
		return here && !isFinished(here) ? here : currentStep(run);
	});
	// Timed reps log their sets only when the run is next open, so only reps
	// and sets have a count to trust here.
	const where = $derived(
		step?.kind === 'reps_and_sets' && step.sets
			? `set ${Math.min(setsOf(run!, step.id).length + 1, step.sets)} of ${step.sets}`
			: ''
	);

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
		// Today always reopens on today, not on a day picked from its strip.
		const at = from.url.pathname === '/' ? '/' : from.url.pathname + from.url.search;
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
		} else if (here === '/' && page.url.search) {
			await goto('/', { replaceState: true });
		} else if (t.roots.includes(here)) {
			scrollTo({ top: 0, behavior: matchMedia('(prefers-reduced-motion: reduce)').matches ? 'instant' : 'smooth' });
		} else {
			await goto(t.roots.find((r) => under(here, r)) ?? t.href);
		}
	}
</script>

<nav
	class="fixed inset-x-0 bottom-0 z-30 mx-auto w-full max-w-[430px]"
	style:view-transition-name="tabbar"
	aria-label="Main"
>
	{#if live}
		<a
			href={step ? `/run/${live.id}/step/${step.id}` : `/run/${live.id}`}
			class="mx-3 mb-2 flex h-15 items-center gap-3 rounded-[22px] bg-hero py-2 pr-2 pl-4 text-on-hero shadow-[0_14px_30px_-10px_rgba(21,32,26,0.55)] dark:shadow-[0_14px_30px_-10px_rgba(0,0,0,0.7),inset_0_0_0_1px_rgba(255,255,255,0.08)]"
			aria-label="Back to {[step?.name, where, live.name].filter(Boolean).join(', ')}"
		>
			<span class="size-2.5 shrink-0 rounded-full bg-live shadow-[0_0_0_4px_var(--live-halo)] motion-safe:animate-pulse"></span>
			<span class="min-w-0 flex-1">
				<b class="block truncate text-[15px] font-bold">{step ? [step.name, where].filter(Boolean).join(' · ') : live.name}</b>
				{#if step}<span class="block truncate text-xs font-semibold text-on-hero-2">{live.name}</span>{/if}
			</span>
			<span class="shrink-0 font-[family-name:var(--font-digits)] text-xl font-bold {restLeft > 0 ? 'text-live' : ''}">
				{restLeft > 0 ? formatClock(restLeft) : sessionClock(secondsSince(live.started_at, now))}
			</span>
			<span class="flex size-11 shrink-0 items-center justify-center rounded-full bg-live text-on-live">
				<Icon name="chevron-right" size="1.25rem" stroke={2.6} />
			</span>
		</a>
	{/if}
	<div
		class="rounded-t-[28px] bg-[var(--bar)] pb-[env(safe-area-inset-bottom)] shadow-[0_-10px_30px_-12px_rgba(21,32,26,0.14)] backdrop-blur-xl dark:shadow-[0_-10px_30px_-10px_rgba(0,0,0,0.6),inset_0_1px_0_rgba(255,255,255,0.05)]"
	>
		<div class="relative grid h-16 grid-cols-4 items-start px-3 pt-2">
			{#if active >= 0}
				<span
					class="pointer-events-none absolute top-2 left-3 flex w-[calc((100%-1.5rem)/4)] justify-center transition-transform duration-300 ease-[cubic-bezier(0.2,0.8,0.2,1)] motion-reduce:hidden"
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
					<span class="relative flex h-8 w-14 items-center justify-center rounded-full transition-colors duration-300 {i === active ? 'text-on-tint' : ''}">
						<!-- With Reduce Motion on, the pill fades from tab to tab instead. -->
						<span
							class="absolute inset-0 hidden rounded-full bg-tint transition-opacity duration-200 motion-reduce:block {i === active ? 'opacity-100' : 'opacity-0'}"
							aria-hidden="true"
						></span>
						<span class="relative"><Icon name={t.icon} size="1.375rem" /></span>
					</span>
					{t.label}
				</a>
			{/each}
		</div>
	</div>
</nav>
