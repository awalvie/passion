<script lang="ts">
	import logo from '$lib/assets/logo.svg?raw';
	import Button from '$lib/Button.svelte';
	import HeroCard from '$lib/HeroCard.svelte';
	import Icon from '$lib/Icon.svelte';
	import Topo from '$lib/Topo.svelte';
	import { topTopo } from '$lib/topo';

	// The same setting as the theme on Profile, so the app opens in the theme picked here.
	let dark = $state(document.documentElement.getAttribute('data-theme') === 'dark');
	function toggleTheme() {
		dark = !dark;
		localStorage.setItem('passion-theme', dark ? 'dark' : 'light');
		document.documentElement.setAttribute('data-theme', dark ? 'dark' : 'light');
	}

	const repo = 'https://github.com/awalvie/passion';
	const links = [
		{ label: 'Source code', href: repo },
		{ label: 'Run it yourself', href: `${repo}/blob/master/docs/DEPLOY.md` }
	];

	const theme = $derived(dark ? 'dark' : 'light');
	const cards = [
		{ title: 'Plan', text: 'Write your own sessions or start from the built-in ones. Pick the days once and they repeat for up to a year.', shot: 'cycle', alt: 'A cycle calendar for October and November' },
		{ title: 'Progress', text: 'Your hours, climbs and grades over time, and the history of each exercise.', shot: 'progress', alt: 'Progress for climbs: climbs a week, send rate and flash rate' },
		{ title: 'Journal', text: 'Pin the notes you want to see when a session starts.', shot: 'journal', alt: 'The Journal with two pinned notes' },
		{ title: 'History', text: 'Each session with its sets, climbs and notes.', shot: 'history', alt: 'History with time, climbs and hardest send this week' }
	];
</script>

<svelte:head>
	<title>Passion: the training app built for climbing</title>
	<meta name="description" content="The training app built for climbing. Time your hangs, log every attempt and send, and plan your weeks. Free, and the code is public." />
</svelte:head>

{#snippet phone(src: string, alt: string)}
	<div class="h-[330px] w-full max-w-[250px] overflow-hidden rounded-t-[40px] bg-run px-[7px] pt-[7px] shadow-card">
		<img {src} {alt} width="786" height="1080" loading="lazy" class="w-full rounded-t-[33px]" />
	</div>
{/snippet}

{#snippet copy(title: string, text: string, muted: string)}
	<div class="flex flex-col gap-2 px-7 pt-7">
		<h3 class="text-xl font-extrabold tracking-[-0.02em]">{title}</h3>
		<p class="max-w-[34ch] text-base leading-relaxed {muted}">{text}</p>
	</div>
{/snippet}

<div class="relative overflow-hidden">
	<div
		class="absolute inset-x-0 top-0 h-[760px] bg-[radial-gradient(120%_70%_at_100%_-10%,var(--glow-1),transparent_62%),radial-gradient(90%_60%_at_-10%_18%,var(--glow-2),transparent_62%)]"
	></div>
	<Topo
		shape={topTopo}
		class="absolute inset-x-0 top-0 h-[760px] w-full text-[var(--topo)] [mask-image:linear-gradient(#000_40%,transparent)] [&_path]:[vector-effect:non-scaling-stroke]"
	/>

	<div
		class="relative mx-auto max-w-[1184px] pt-[env(safe-area-inset-top)] pr-[max(1rem,env(safe-area-inset-right))] pl-[max(1rem,env(safe-area-inset-left))] md:pr-[max(2.5rem,env(safe-area-inset-right))] md:pl-[max(2.5rem,env(safe-area-inset-left))]"
	>
		<header class="flex h-22 items-center justify-between">
			<span class="flex items-center gap-2.5 text-[28px] font-extrabold tracking-[-0.03em] [&>svg]:h-11! [&>svg]:w-auto!">
				{@html logo}Passion
			</span>
			<div class="flex items-center gap-2">
				<button
					type="button"
					onclick={toggleTheme}
					aria-label={dark ? 'Light theme' : 'Dark theme'}
					class="grid size-11 place-items-center rounded-full bg-surface shadow-card-sm"
				>
					<Icon name={dark ? 'sun' : 'moon'} size="1.25rem" />
				</button>
				<div class="w-24"><Button variant="secondary" href="/login">Log in</Button></div>
			</div>
		</header>

		<main>
			<section class="flex flex-col items-center gap-4 pt-4 text-center md:pt-12">
				<h1 class="max-w-[16ch] text-[clamp(44px,6.4vw,88px)] leading-none font-extrabold tracking-[-0.02em] text-balance">The training app built for climbing</h1>
				<p class="max-w-[40ch] text-[clamp(17px,1.6vw,20px)] leading-relaxed text-balance text-ink-2">
					Time your hangs, log every attempt and send, and plan your weeks.
				</p>
				<div class="mt-2 flex flex-col items-center gap-3">
					<div class="w-60"><Button href="/signup">Sign up</Button></div>
					<p class="text-[15px] font-semibold text-ink-2">Free. No ads, no subscription.</p>
				</div>
			</section>

			<!-- A phone gets the climb log, a wider screen the laptop, and only that one downloads. -->
			<div class="mt-10 flex justify-center md:mt-14">
				<picture class="block w-60 overflow-hidden rounded-[40px] bg-run p-[7px] shadow-card md:w-full md:rounded-[20px] md:bg-surface md:p-0 md:ring-1 md:ring-line">
					<source media="(min-width: 768px)" srcset="/landing/laptop-today-{theme}.png" width="2880" height="1630" />
					<img src="/landing/climbs.png" width="786" height="1704" alt="The app: the week on a laptop, the climb log on a phone" class="w-full rounded-[33px] md:rounded-none" />
				</picture>
			</div>

			<!-- On a phone the cards are one row you swipe through. -->
			<!-- svelte-ignore a11y_no_noninteractive_tabindex -->
			<section
				aria-label="Features"
				tabindex="0"
				class="-mx-[max(1rem,env(safe-area-inset-left))] grid snap-x snap-mandatory auto-cols-[86%] grid-flow-col gap-5 overflow-x-auto px-[max(1rem,env(safe-area-inset-left))] pt-12 pb-2 [scroll-padding-inline:max(1rem,env(safe-area-inset-left))] md:-mx-10 md:px-10 lg:mx-0 lg:grid-flow-row lg:grid-cols-3 lg:overflow-visible lg:px-0 lg:pt-24"
			>
				<HeroCard flush class="flex min-h-[500px] snap-start flex-col lg:col-span-2 lg:min-h-[520px]">
					{@render copy('Train', "Time hangs and repeaters. Log each go on a climb as an attempt or a send. Last time's reps and weight show under each set.", 'text-on-hero-2')}
					<div class="mt-auto flex justify-center gap-5 px-7 pt-6">
						{@render phone('/landing/timer.png', 'A hang timer: work, rep 1 of 6, eight seconds left')}
						<div class="hidden lg:contents">
							{@render phone('/landing/sets.png', "Deadlift sets with last time's reps and weight under each")}
						</div>
					</div>
				</HeroCard>
				{#each cards as c (c.title)}
					<article class="flex min-h-[500px] snap-start flex-col overflow-hidden rounded-[32px] bg-surface shadow-card lg:min-h-[520px]">
						{@render copy(c.title, c.text, 'text-ink-2')}
						<div class="mt-auto flex justify-center px-7 pt-6">
							{@render phone(`/landing/${c.shot}-${theme}.png`, c.alt)}
						</div>
					</article>
				{/each}
			</section>

			<section class="mx-auto flex max-w-[480px] flex-col items-center gap-5 pt-16 lg:pt-24">
				<h2 class="text-[clamp(28px,3vw,36px)] leading-tight font-extrabold tracking-[-0.02em]">The code is public</h2>
				<ul class="w-full overflow-hidden rounded-3xl bg-surface shadow-card">
					{#each links as l (l.href)}
						<li class="list-row">
							<a href={l.href} class="flex min-h-14 items-center justify-between px-5 text-[15px] font-bold">
								{l.label}<span class="text-ink-2"><Icon name="chevron-right" size="1.125rem" /></span>
							</a>
						</li>
					{/each}
				</ul>
			</section>

			<HeroCard class="mt-16 lg:mt-24">
				<div class="flex flex-col items-center gap-6 py-6 text-center lg:py-12">
					<h2 class="text-[clamp(32px,4vw,48px)] leading-tight font-extrabold tracking-[-0.02em]">Create an account</h2>
					<div class="w-full lg:w-60"><Button href="/signup">Sign up</Button></div>
				</div>
			</HeroCard>
		</main>

		<footer class="mt-6 px-1 pb-[calc(2rem+env(safe-area-inset-bottom))] text-xs text-ink-2">Passion · PolyForm Noncommercial licence</footer>
	</div>
</div>
