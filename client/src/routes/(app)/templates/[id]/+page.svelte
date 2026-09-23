<script lang="ts">
	import { goto } from '$app/navigation';
	import { describe, request } from '$lib/api';
	import { fieldLabel } from '$lib/exercise';
	import FormError from '$lib/FormError.svelte';
	import Icon from '$lib/Icon.svelte';
	import Notes from '$lib/Notes.svelte';
	import TemplatePlan from '$lib/TemplatePlan.svelte';
	import type { SessionTemplate } from '$lib/template';

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
			error = describe(e, fieldLabel);
		} finally {
			busy = false;
		}
	}

	async function retire() {
		if (!confirm(`Retire “${t.name}”? It leaves your list of session templates.`)) return;
		error = '';
		busy = true;
		try {
			await request('POST', `/api/v1/session-templates/${t.id}/retire`);
			await goto('/templates');
		} catch (e) {
			error = describe(e, fieldLabel);
		} finally {
			busy = false;
		}
	}
</script>

<svelte:head><title>{t.name}</title></svelte:head>

<div class="space-y-4">
	<div class="card card-pad">
		<div class="flex items-start gap-2">
			{#if t.color}
				<span class="inline-block w-3 h-3 rounded-full shrink-0 mt-2" style="background:{t.color}"></span>
			{/if}
			<div class="min-w-0 flex-1">
				<h1 class="text-xl font-bold m-0 break-words">{t.name}</h1>
				{#if t.tags.length}
					<div class="mt-1 text-[11px] muted">{t.tags.join(' · ')}</div>
				{/if}
			</div>
			<a
				class="rounded-md btn-ghost p-2 inline-flex items-center justify-center shrink-0"
				href="/templates"
				title="Back to templates"
				aria-label="Back to templates"
			>
				<Icon name="arrow-left" />
			</a>
		</div>

		{#if locked}
			<p class="mt-2 text-xs muted">{locked}</p>
		{/if}

		{#if t.source || t.needs}
			<div class="mt-3 flex flex-wrap items-center gap-2 text-[11px] muted">
				{#if t.source}
					<span
						class="inline-flex items-center gap-1 font-medium px-1.5 py-0.5 rounded"
						style="background:var(--accent-bg);color:var(--accent)"
					>
						<Icon name="book-marked" size="0.6rem" />{t.source}
					</span>
				{/if}
				{#if t.needs}
					<span>Needs: {t.needs}</span>
				{/if}
			</div>
		{/if}

		{#if t.notes}
			<Notes text={t.notes} class="mt-3 text-sm" />
		{/if}

		<!-- Beside the title, these would squeeze a long name onto several lines on a phone. -->
		<div class="mt-3 pt-3 divider flex flex-wrap items-center gap-2">
			{#if !locked}
				<a
					class="rounded-md btn-ghost px-3 py-2 text-sm inline-flex items-center gap-1.5"
					href="/templates/{t.id}/edit"
				>
					<Icon name="pencil" size="0.875rem" />
					Edit
				</a>
			{/if}
			<button
				type="button"
				class="rounded-md btn-ghost px-3 py-2 text-sm inline-flex items-center gap-1.5"
				disabled={busy}
				onclick={duplicate}
			>
				<Icon name="copy" size="0.875rem" />
				Duplicate
			</button>
			{#if !locked}
				<button
					type="button"
					class="rounded-md btn-ghost px-3 py-2 text-sm inline-flex items-center gap-1.5"
					style="color:var(--destructive)"
					disabled={busy}
					onclick={retire}
				>
					<Icon name="archive" size="0.875rem" />
					Retire
				</button>
			{/if}
		</div>
		<FormError message={error} />
	</div>

	<TemplatePlan sections={t.sections} />
</div>
