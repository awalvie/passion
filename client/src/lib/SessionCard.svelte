<script lang="ts">
	import Button from './Button.svelte';
	import Icon from './Icon.svelte';
	import type { ScheduledDay } from './plan';
	import Sheet from './Sheet.svelte';
	import type { SessionTemplate } from './template';
	import Topo from './Topo.svelte';
	import TemplatePlan from './TemplatePlan.svelte';
	import { heroTopo } from './topo';

	let {
		day,
		template,
		label,
		starting,
		busy,
		startLabel = 'Start',
		onstart
	}: {
		day: ScheduledDay;
		template: SessionTemplate | undefined;
		label: string;
		starting: boolean;
		// This card's start is the one waiting.
		busy: boolean;
		startLabel?: string;
		onstart: () => void;
	} = $props();

	const live = $derived(day.status === 'started' && !!day.run);
	const sections = $derived(template?.sections ?? []);

	let previewing = $state(false);
</script>

<article
	class="relative mx-4 mt-3.5 overflow-hidden rounded-[32px] bg-[radial-gradient(90%_70%_at_88%_0%,var(--hero-2),var(--hero)_70%)] text-on-hero shadow-[0_18px_36px_-12px_rgba(10,30,18,0.55)]"
>
	<Topo shape={heroTopo} class="absolute inset-0 h-full w-full text-[var(--hero-topo)]" />
	<div class="relative flex flex-col p-5">
		{#if live}
			<p class="flex items-center gap-2.5 text-xs font-semibold tracking-[0.06em] text-on-hero uppercase">
				<i class="size-2.5 rounded-full bg-live shadow-[0_0_0_5px_var(--live-halo)]"></i>Live
			</p>
		{:else}
			<p class="text-xs font-semibold tracking-[0.06em] text-on-hero-2 uppercase">{label}</p>
		{/if}
		<h2 class="mt-1 text-[32px] leading-tight font-extrabold tracking-tight">{day.template_name}</h2>
		{#if sections.length || template?.needs}
			<div class="mt-3 flex flex-wrap items-center gap-x-5 gap-y-2 text-xl font-bold tracking-tight">
				{#if sections.length}
					<p class="flex items-center gap-2">
						<span class="text-tint opacity-90"><Icon name="layers2" size="1.25rem" /></span>
						{sections.length} section{sections.length === 1 ? '' : 's'}
					</p>
				{/if}
				{#if template?.needs}
					<p class="flex min-w-0 items-center gap-2">
						<span class="shrink-0 text-tint opacity-90"><Icon name="backpack" size="1.25rem" /></span>
						{template.needs}
					</p>
				{/if}
			</div>
		{/if}
		<div class="mt-4 flex gap-2.5">
			{#if !live && template}
				<button
					type="button"
					class="flex h-14 shrink-0 items-center gap-2 rounded-full bg-white/10 px-5 text-[15px] font-bold text-on-hero"
					onclick={() => (previewing = true)}
				>
					<Icon name="list" size="1.25rem" />Preview
				</button>
			{/if}
			<div class="flex-1">
				{#if live}
					<Button variant="live" href="/run/{day.run}">Back to session</Button>
				{:else}
					<Button disabled={starting} onclick={onstart}>{busy ? 'Starting…' : startLabel}</Button>
				{/if}
			</div>
		</div>
	</div>
</article>

{#if template}
	<Sheet bind:open={previewing} eyebrow={label} title={template.name}>
		<p class="mx-1 flex items-center gap-2 text-xl font-bold tracking-tight">
			<span class="text-ink-2"><Icon name="layers2" size="1.25rem" /></span>
			{sections.length} section{sections.length === 1 ? '' : 's'}
		</p>
		{#if template.needs}
			<p class="mx-1 mt-2 mb-1 text-[15px] leading-snug font-semibold text-ink-2">{template.needs}</p>
		{/if}
		<TemplatePlan sections={template.sections} />
		<div class="mt-[22px]">
			<Button
				disabled={starting}
				onclick={() => {
					previewing = false;
					onstart();
				}}
			>
				Start {template.name}
			</Button>
		</div>
	</Sheet>
{/if}
