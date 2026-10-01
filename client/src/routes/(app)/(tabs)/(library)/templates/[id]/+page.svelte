<script lang="ts">
	import { goto } from '$app/navigation';
	import { describe, request } from '$lib/api';
	import Button from '$lib/Button.svelte';
	import FormError from '$lib/FormError.svelte';
	import Icon from '$lib/Icon.svelte';
	import Menu, { type MenuItem } from '$lib/Menu.svelte';
	import NavBar from '$lib/NavBar.svelte';
	import Notes from '$lib/Notes.svelte';
	import TemplatePlan from '$lib/TemplatePlan.svelte';
	import { startRun } from '$lib/runState.svelte';
	import { templateFieldLabel, type SessionTemplate } from '$lib/template';

	let { data } = $props();

	const t = $derived(data.template);
	const locked = $derived(
		t.shipped
			? 'The app ships this template, so it cannot be changed. Duplicate it to make your own.'
			: t.retired_at
				? 'You retired this template.'
				: ''
	);

	let error = $state('');
	let busy = $state(false);

	// A copy is a new template of your own. Its steps keep the exercise ids, so
	// both copies still count toward the same exercises.
	async function duplicate() {
		if (busy) return;
		error = '';
		busy = true;
		try {
			const { id, shipped, retired_at, ...fields } = t;
			const copy = await request<SessionTemplate>('POST', '/api/v1/session-templates', {
				...fields,
				name: `${t.name} (copy)`
			});
			await goto(`/templates/${copy.id}`);
		} catch (e) {
			error = describe(e, templateFieldLabel);
		} finally {
			busy = false;
		}
	}

	async function start() {
		error = '';
		busy = true;
		try {
			const run = await startRun({ template: t.id });
			await goto(`/run/${run.id}`);
		} catch (e) {
			error = describe(e, templateFieldLabel);
		} finally {
			busy = false;
		}
	}

	async function retire() {
		if (busy) return;
		if (!confirm(`Retire “${t.name}”? It leaves your list of session templates.`)) return;
		error = '';
		busy = true;
		try {
			await request('POST', `/api/v1/session-templates/${t.id}/retire`);
			await goto('/templates');
		} catch (e) {
			error = describe(e, templateFieldLabel);
		} finally {
			busy = false;
		}
	}

	const menu = $derived<MenuItem[]>([
		{ label: 'Duplicate', onclick: duplicate },
		...(locked ? [] : [{ label: 'Retire', danger: true, onclick: retire }])
	]);
</script>

<svelte:head><title>{t.name}</title></svelte:head>

<NavBar back={{ href: '/templates', label: 'Sessions' }}>
	{#snippet actions()}
		{#if !locked}
			<a
				href="/templates/{t.id}/edit"
				class="flex h-11 items-center gap-1.5 rounded-full bg-surface px-4 text-[15px] font-bold text-ink shadow-card-sm"
			>
				<Icon name="pencil" size="0.875rem" />
				Edit
			</a>
		{/if}
		<Menu items={menu} />
	{/snippet}
</NavBar>

<div class="flex flex-col gap-3.5 px-4 pt-2">
	<header class="px-1">
		<div class="flex items-start gap-2.5">
			{#if t.color}
				<span class="mt-3.5 size-3 shrink-0 rounded-full" style="background:{t.color}" aria-hidden="true"></span>
			{/if}
			<h1 class="min-w-0 text-[32px] leading-[1.1] font-extrabold tracking-[-0.02em] break-words">{t.name}</h1>
		</div>
		{#if t.tags.length}
			<p class="mt-1 text-[15px] font-semibold text-ink-2">{t.tags.join(' · ')}</p>
		{/if}
		{#if t.source || t.needs}
			<div class="mt-3 flex flex-wrap items-center gap-2">
				{#if t.source}
					<span class="inline-flex items-center gap-1 rounded-xl bg-well px-2.5 py-[5px] text-xs font-bold text-ink-2">
						<Icon name="book-marked" size="0.75rem" />{t.source}
					</span>
				{/if}
				{#if t.needs}
					<span class="rounded-xl px-2.5 py-[5px] text-xs font-bold text-ink-2 shadow-[inset_0_0_0_1.5px_var(--well)]">Needs: {t.needs}</span>
				{/if}
			</div>
		{/if}
		{#if locked}
			<p class="mt-3 text-xs font-semibold text-ink-2">{locked}</p>
		{/if}
	</header>

	{#if t.notes}
		<Notes text={t.notes} class="rounded-[18px] bg-well px-4 py-3.5 text-[15px] font-semibold dark:bg-surface" />
	{/if}

	<Button disabled={busy} onclick={start}>
		<Icon name="play" />
		Start
	</Button>
	<FormError message={error} />

	<TemplatePlan sections={t.sections} />
</div>
