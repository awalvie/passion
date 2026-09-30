<script lang="ts">
	import Button from '$lib/Button.svelte';
	import Icon from '$lib/Icon.svelte';
	import type { ScheduledDay } from '$lib/plan';

	let { data } = $props();

	const date = $derived(
		new Date(`${data.today}T12:00:00`).toLocaleDateString(undefined, {
			weekday: 'long',
			day: 'numeric',
			month: 'long'
		})
	);

	function facts(d: ScheduledDay): string {
		const t = data.templates.get(d.template);
		if (!t) return '';
		const n = t.sections.length;
		return [`${n} section${n === 1 ? '' : 's'}`, t.needs].filter(Boolean).join(' · ');
	}
</script>

<svelte:head><title>Today</title></svelte:head>

<div class="flex flex-col gap-4 px-4 pt-[calc(env(safe-area-inset-top)+0.75rem)]">
	<header class="flex items-start justify-between gap-3">
		<div>
			<p class="text-sm text-ink-2">{date}</p>
			<h1 class="text-3xl font-bold">Today</h1>
		</div>
		<a href="/settings" class="flex size-11 items-center justify-center text-ink-2" aria-label="Settings">
			<Icon name="settings" size="1.5rem" />
		</a>
	</header>

	{#each data.days as d (d.id)}
		{@const t = data.templates.get(d.template)}
		<article class="flex flex-col gap-4 rounded-2xl bg-surface p-4 shadow-sm">
			<a href="/templates/{d.template}" class="flex flex-col gap-1">
				<h2 class="text-xl font-semibold">{d.template_name}</h2>
				<p class="text-base text-ink-2">{facts(d)}</p>
			</a>
			{#if t}
				<ol class="flex flex-wrap gap-x-4 gap-y-1 text-sm text-ink-2">
					{#each t.sections as s, i (i)}
						<li class="flex items-center gap-1.5">
							<span class="size-2 rounded-full bg-tint"></span>{s.name}
						</li>
					{/each}
				</ol>
			{/if}
			{#if d.status === 'done'}
				<p class="flex items-center gap-2 text-base font-semibold text-tint">
					<Icon name="check" size="1.25rem" />Done
				</p>
			{:else if d.status === 'started' && d.run}
				<Button variant="live" href="/run/{d.run}">Resume</Button>
			{/if}
			<a href="/templates/{d.template}" class="text-center text-base text-tint">See the whole session</a>
		</article>
	{:else}
		<section class="flex flex-col gap-3 rounded-2xl bg-surface p-4 shadow-sm">
			<p class="text-base">Nothing planned today.</p>
			<Button variant="secondary" href="/templates">Pick a session</Button>
		</section>
	{/each}
</div>
