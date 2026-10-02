<script lang="ts">
	import { goto } from '$app/navigation';
	import { describe, request } from '$lib/api';
	import Button from '$lib/Button.svelte';
	import { saveCentre, type Centre } from '$lib/centres';
	import { haptic } from '$lib/haptics';
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
	let doing = $state<'duplicate' | 'start' | 'retire' | null>(null);

	// A copy is a new template of your own. Its steps keep the exercise ids, so
	// both copies still count toward the same exercises.
	async function duplicate() {
		if (doing) return;
		error = '';
		doing = 'duplicate';
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
			doing = null;
		}
	}

	async function start() {
		error = '';
		doing = 'start';
		try {
			const run = await startRun({ template: t.id });
			await goto(`/run/${run.id}`);
		} catch (e) {
			error = describe(e, templateFieldLabel);
		} finally {
			doing = null;
		}
	}

	async function retire() {
		if (doing) return;
		if (!confirm(`Retire “${t.name}”? It leaves your list of session templates.`)) return;
		error = '';
		doing = 'retire';
		try {
			await request('POST', `/api/v1/session-templates/${t.id}/retire`);
			await goto('/templates');
		} catch (e) {
			error = describe(e, templateFieldLabel);
		} finally {
			doing = null;
		}
	}

	// A tap on a centre says whether this session can be done there.
	let centres = $state<Centre[]>([]);
	$effect.pre(() => {
		centres = structuredClone($state.snapshot(data.centres));
	});

	async function toggleCentre(i: number) {
		const before = $state.snapshot(centres[i]);
		const on = before.sessions.includes(t.id);
		centres[i].sessions = on ? before.sessions.filter((s) => s !== t.id) : [...before.sessions, t.id];
		error = '';
		try {
			await saveCentre($state.snapshot(centres[i]));
		} catch (e) {
			centres[i] = before;
			error = describe(e, () => 'That centre');
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

	<Button disabled={doing !== null} onclick={start}>
		<Icon name="play" />
		{doing === 'start' ? 'Starting…' : 'Start'}
	</Button>
	<FormError message={error} />

	<section class="flex flex-col gap-2">
		<h2 class="px-1 text-[15px] font-bold">Where you can do it</h2>
		{#if centres.length}
			<div class="flex flex-wrap gap-2">
				{#each centres as c, i (c.id)}
					{@const on = c.sessions.includes(t.id)}
					<button
						type="button"
						class="h-11 max-w-full truncate rounded-full px-4 text-[15px] font-bold {on ? 'bg-ink text-ground' : 'bg-surface text-ink shadow-card-sm'}"
						aria-pressed={on}
						onclick={() => toggleCentre(i)}
						use:haptic
					>
						{c.name}
					</button>
				{/each}
			</div>
		{:else}
			<a href="/profile" class="flex min-h-11 items-center px-1 text-xs font-semibold text-ink-2">Add your climbing centres in Profile</a>
		{/if}
	</section>

	<TemplatePlan sections={t.sections} />
</div>
