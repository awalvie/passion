<script lang="ts">
	import logo from '$lib/assets/logo.svg?raw';
	import Button from '$lib/Button.svelte';
	import HeroCard from '$lib/HeroCard.svelte';
	import Icon from '$lib/Icon.svelte';
	import Topo from '$lib/Topo.svelte';
	import { topTopo } from '$lib/topo';

	const repo = 'https://github.com/awalvie/passion';
	const links = [
		{ label: 'See the code', href: repo },
		{ label: 'See every change', href: `${repo}/commits/master` },
		{ label: 'Host it yourself', href: `${repo}/blob/master/docs/DEPLOY.md` }
	];
</script>

<svelte:head>
	<title>Passion: the training app built for climbing</title>
	<meta name="description" content="The training app built for climbing. Time your hangs, log every attempt and send, and plan your weeks. Free, and the code is public." />
</svelte:head>

{#snippet card(title: string, text: string)}
	<div class="flex flex-col gap-2 px-6 pt-6 pb-5">
		<h2 class="text-[28px] leading-tight font-extrabold tracking-[-0.02em]">{title}</h2>
		<p class="text-[15px] leading-relaxed text-ink-2">{text}</p>
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
		class="relative mx-auto max-w-[1104px] pt-[env(safe-area-inset-top)] pr-[max(1rem,env(safe-area-inset-right))] pl-[max(1rem,env(safe-area-inset-left))] md:pr-[max(2.5rem,env(safe-area-inset-right))] md:pl-[max(2.5rem,env(safe-area-inset-left))]"
	>
		<header class="flex h-18 items-center justify-between">
			<span class="flex items-center gap-2 text-xl font-extrabold tracking-tight [&>svg]:h-[30px]! [&>svg]:w-auto!">
				{@html logo}Passion
			</span>
			<a href="/login" class="flex h-11 items-center rounded-full bg-surface px-[18px] text-[15px] font-bold shadow-card-sm">Log in</a>
		</header>

		<main>
			<div class="grid gap-6 pt-6 lg:grid-cols-[7fr_5fr] lg:items-center lg:gap-12 lg:pt-12">
				<div class="flex flex-col items-start gap-4">
					<h1 class="text-[40px] leading-[1.05] font-extrabold tracking-[-0.02em] text-balance">The training app built for climbing</h1>
					<p class="max-w-[44ch] text-[15px] leading-relaxed font-medium text-pretty text-ink-2">
						Time your hangs, log every attempt and send, and plan your weeks.
					</p>
					<div class="mt-1 flex w-full flex-col gap-2.5 text-center lg:w-60 lg:text-left">
						<Button href="/signup">Sign up</Button>
						<p class="text-[15px] font-semibold lg:whitespace-nowrap">Free. No ads, no subscription. Cross-platform.</p>
					</div>
				</div>
				<div class="flex justify-center max-lg:h-[350px] max-lg:overflow-hidden max-lg:pt-1 max-lg:[mask-image:linear-gradient(#000_85%,transparent)]">
					<img
						src="/landing/climbs.png"
						width="1000"
						height="1534"
						alt="Logging a boulder: grade buttons from 6b to 7a+, then Attempt and Send"
						class="w-[82%] rounded-[28px] shadow-card ring-1 ring-line lg:w-[300px]"
					/>
				</div>
			</div>

			<!-- Each shot sits in a box of one height at the foot of its card. A tall
			     shot fades out before its next row shows. -->
			<ul class="grid gap-6 pt-12 lg:grid-cols-3 lg:pt-18">
				<li class="flex min-w-0 flex-col overflow-hidden rounded-3xl bg-surface shadow-card">
					{@render card('Plan', 'Write your own sessions or start from the built-in ones. Put them on a calendar. Pick the days once and they repeat for up to a year.')}
					<div class="mt-auto h-[340px] overflow-hidden px-6 [mask-image:linear-gradient(#000_72%,transparent_97%)]">
						<div class="mx-auto max-w-[300px] overflow-hidden rounded-t-[20px] ring-1 ring-line">
							<img src="/landing/cycle-light.png" width="796" height="1265" loading="lazy" alt="A cycle calendar with done, missed and planned sessions, and today" class="w-full dark:hidden" />
							<img src="/landing/cycle-dark.png" width="796" height="1265" loading="lazy" alt="A cycle calendar with done, missed and planned sessions, and today" class="hidden w-full dark:block" />
						</div>
					</div>
				</li>
				<li class="flex min-w-0 flex-col overflow-hidden rounded-3xl bg-surface shadow-card">
					{@render card('Train', "Time hangs and repeaters. Log each go on a climb as an attempt or a send. Last time's reps and weight show under each set.")}
					<div class="mt-auto h-[340px] overflow-hidden px-6 [mask-image:linear-gradient(#000_72%,transparent_97%)]">
						<div class="mx-auto max-w-[300px] overflow-hidden rounded-t-[20px] ring-1 ring-line">
							<img src="/landing/timer.png" width="1000" height="1534" loading="lazy" alt="A hang timer: work, rep 1 of 6, four seconds left" class="w-full" />
						</div>
					</div>
				</li>
				<li class="flex min-w-0 flex-col overflow-hidden rounded-3xl bg-surface shadow-card">
					{@render card('Progress', 'A grade pyramid of your sends with flashes marked, and your send and flash rates. Show grades in Font, V scale, French or YDS.')}
					<div class="mt-auto h-[340px] px-6">
						<div class="mx-auto max-w-[300px] overflow-hidden rounded-[20px] ring-1 ring-line">
							<img src="/landing/pyramid-light.png" width="796" height="809" loading="lazy" alt="A grade pyramid of 46 boulders sent, from 6b to 7b" class="w-full dark:hidden" />
							<img src="/landing/pyramid-dark.png" width="796" height="809" loading="lazy" alt="A grade pyramid of 46 boulders sent, from 6b to 7b" class="hidden w-full dark:block" />
						</div>
					</div>
				</li>
			</ul>

			<section class="flex flex-col gap-2 px-1 pt-12 lg:pt-18">
				<h2 class="text-[28px] leading-tight font-extrabold tracking-[-0.02em]">Free, and the code is public</h2>
				<p class="max-w-[52ch] text-[15px] leading-relaxed text-ink-2">
					The code changes every week. Read it, report a bug or host it on your own server.
				</p>
				<div class="flex flex-wrap gap-x-6">
					{#each links as l (l.href)}
						<a href={l.href} class="flex min-h-11 items-center gap-1 text-[15px] font-bold">
							{l.label}<span class="text-ink-2"><Icon name="chevron-right" size="1.125rem" /></span>
						</a>
					{/each}
				</div>
			</section>

			<HeroCard class="mt-12 lg:mt-18">
				<div class="flex flex-col items-center gap-3 py-5 text-center lg:py-10">
					<h2 class="page-title">Create an account</h2>
					<p class="text-[15px] text-on-hero-2">Plan your week or start a session.</p>
					<div class="mt-3 w-full lg:w-60"><Button href="/signup">Sign up</Button></div>
				</div>
			</HeroCard>
		</main>

		<footer class="mt-6 px-1 pb-[calc(2rem+env(safe-area-inset-bottom))] text-xs text-ink-2">Passion · PolyForm Noncommercial licence</footer>
	</div>
</div>
