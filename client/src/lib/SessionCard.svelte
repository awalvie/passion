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
		onstart
	}: {
		day: ScheduledDay;
		template: SessionTemplate | undefined;
		label: string;
		starting: boolean;
		onstart: () => void;
	} = $props();

	// The dots climb and dip like the design's trail, with no line between them.
	const heights = [24, 6, 18, 0, 14];

	const live = $derived(day.status === 'started' && !!day.run);
	const sections = $derived(template?.sections ?? []);
	const shown = $derived(sections.length > 5 ? sections.slice(0, 4) : sections);
	const more = $derived(sections.length - shown.length);

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
		{#if sections.length}
			<p class="mt-3 flex items-center gap-2 text-xl font-bold tracking-tight">
				<span class="text-tint opacity-90"><Icon name="layers2" size="1.25rem" /></span>
				{sections.length} section{sections.length === 1 ? '' : 's'}
			</p>
			<ol class="mt-3 grid h-16" style="grid-template-columns: repeat({shown.length + (more ? 1 : 0)}, minmax(0, 1fr))">
				{#each shown as s, i (i)}
					<li class="flex min-w-0 flex-col items-center">
						<span class="size-3 shrink-0 rounded-full border-2 border-on-hero bg-white/15" style="margin-top: {heights[i % heights.length]}px"></span>
						<span class="mt-auto w-full truncate text-center text-xs font-semibold text-on-hero/70">{s.name}</span>
					</li>
				{/each}
				{#if more}
					<li class="flex flex-col items-center justify-end text-xs font-semibold text-on-hero/70">+{more}</li>
				{/if}
			</ol>
		{/if}
		{#if template?.needs}
			<p class="mt-2.5 text-[15px] leading-snug font-semibold text-on-hero-2">{template.needs}</p>
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
				{#if day.status === 'done'}
					<p class="flex h-14 items-center justify-center gap-2 text-xl font-bold text-tint">
						<Icon name="check" size="1.25rem" />Done
					</p>
				{:else if live}
					<Button variant="live" href="/run/{day.run}">Back to session</Button>
				{:else}
					<Button disabled={starting} onclick={onstart}>Start</Button>
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
		{#if day.status !== 'done'}
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
		{/if}
	</Sheet>
{/if}
