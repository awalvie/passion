<script lang="ts">
	import { untrack } from 'svelte';
	import { beforeNavigate } from '$app/navigation';
	import { page } from '$app/state';
	import { describe, request } from '$lib/api';
	import Button from '$lib/Button.svelte';
	import type { Exercise } from '$lib/exercise';
	import FormError from '$lib/FormError.svelte';
	import Icon from '$lib/Icon.svelte';
	import RowActions from '$lib/RowActions.svelte';
	import SectionEditor from '$lib/SectionEditor.svelte';
	import { sessionIcons } from '$lib/SessionIcon.svelte';
	import TemplatePlan from '$lib/TemplatePlan.svelte';
	import {
		moveEntry,
		templateFieldLabel,
		toTemplateBody,
		toTemplateDraft,
		type SessionTemplate
	} from '$lib/template';

	let {
		template,
		exercises,
		sources,
		save,
		cancel
	}: {
		template?: SessionTemplate;
		exercises: Exercise[];
		sources: string[];
		save: (body: ReturnType<typeof toTemplateBody>) => Promise<void>;
		cancel: string;
	} = $props();

	let draft = $state(untrack(() => toTemplateDraft(template)));

	// The API replaces the whole template, so nothing is kept until Save.
	const clean = JSON.stringify(draft);
	let saving = false;
	beforeNavigate((navigation) => {
		if (saving || JSON.stringify(draft) === clean) return;
		// A reload or a closed tab can only get the browser's own prompt.
		if (navigation.type === 'leave') navigation.cancel();
		else if (!confirm('Discard your unsaved changes?')) navigation.cancel();
	});

	// A missing exercise is made in another tab, so the list reloads when the
	// user comes back to this one.
	let library = $state(untrack(() => exercises));
	async function reload() {
		try {
			library = (await request<{ exercises: Exercise[] }>('GET', '/api/v1/exercises')).exercises;
		} catch {
			/* keep the list it has */
		}
	}
	let newSection = $state('');
	let error = $state('');
	let busy = $state(false);

	const presets = [
		['#6366f1', 'Indigo'],
		['#2563eb', 'Blue'],
		['#059669', 'Green'],
		['#d97706', 'Amber'],
		['#ef4444', 'Red'],
		['#db2777', 'Pink'],
		['#7c3aed', 'Violet']
	];
	const tags = $derived(
		draft.tags
			.split(',')
			.map((t) => t.trim())
			.filter(Boolean)
	);
	const custom = $derived(draft.color !== '' && !presets.some(([c]) => c === draft.color));

	function addSection() {
		const name = newSection.trim();
		if (!name) return;
		draft.sections.push({ name, notes: null, items: [] });
		newSection = '';
	}

	function removeSection(i: number) {
		const n = draft.sections[i].items.length;
		if (n && !confirm('Remove this section and all its exercises?')) return;
		draft.sections.splice(i, 1);
	}

	// A field inside a closed row cannot show the browser's message, so open
	// every row around it first.
	function reveal(event: Event) {
		let row = (event.target as HTMLElement).closest('details');
		while (row) {
			row.open = true;
			row = row.parentElement?.closest('details') ?? null;
		}
	}

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		error = '';
		busy = true;
		saving = true;
		try {
			await save(toTemplateBody(draft));
		} catch (e) {
			saving = false;
			error = describe(e, templateFieldLabel);
		} finally {
			busy = false;
		}
	}
</script>

<svelte:window onfocus={reload} />

<form class="flex flex-col gap-3.5 pt-2" onsubmit={submit} oninvalidcapture={reveal}>
	<header class="flex items-start gap-3 px-1">
		{#if draft.color}
			<span class="mt-3.5 size-3 shrink-0 rounded-full" style="background:{draft.color}"></span>
		{/if}
		<div class="min-w-0 flex-1">
			<h1 class="m-0 text-[32px] leading-[1.1] font-extrabold tracking-[-0.02em] break-words">
				{draft.name || 'New session template'}
			</h1>
			{#if tags.length}
				<div class="mt-1 text-[15px] font-semibold text-ink-2">{tags.join(' · ')}</div>
			{/if}
		</div>
		<a
			class="flex size-10 shrink-0 items-center justify-center rounded-full bg-surface text-ink shadow-card-sm"
			href={cancel}
			title="Cancel"
			aria-label="Cancel"
		>
			<Icon name="arrow-left" size="1.25rem" />
		</a>
	</header>

	<details class="group/settings rounded-3xl bg-surface shadow-card" open={!template}>
		<summary
			class="flex h-14 cursor-pointer list-none items-center gap-2.5 px-[18px] text-[15px] font-bold [&::-webkit-details-marker]:hidden"
		>
			<span class="text-ink-3"><Icon name="pencil" /></span>
			<span class="flex-1">Settings</span>
			<span class="text-ink-3 transition-transform group-open/settings:rotate-90"><Icon name="chevron-right" /></span>
		</summary>
		<div class="flex flex-col gap-3.5 border-t border-line px-[18px] pt-4 pb-[18px]">
			<div>
				<label class="block text-xs font-semibold text-ink-2" for="tpl-name">Name</label>
				<input id="tpl-name" class="mt-1.5 w-full input" maxlength="200" required bind:value={draft.name} />
			</div>
			<div>
				<label class="block text-xs font-semibold text-ink-2" for="tpl-source">Source</label>
				<input
					id="tpl-source"
					list="tpl-sources"
					class="mt-1.5 w-full input"
					placeholder="e.g. Power Company Climbing"
					autocomplete="off"
					bind:value={draft.source}
				/>
				<datalist id="tpl-sources">
					{#each sources as s (s)}
						<option value={s}></option>
					{/each}
				</datalist>
			</div>
			<div>
				<label class="block text-xs font-semibold text-ink-2" for="tpl-tags">Labels</label>
				<input
					id="tpl-tags"
					class="mt-1.5 w-full input"
					placeholder="comma-separated, e.g. technique, indoor"
					autocomplete="off"
					bind:value={draft.tags}
				/>
			</div>
			<div>
				<label class="block text-xs font-semibold text-ink-2" for="tpl-needs">Needs</label>
				<input
					id="tpl-needs"
					class="mt-1.5 w-full input"
					placeholder="equipment, e.g. hangboard, kilter, 20mm edge"
					autocomplete="off"
					bind:value={draft.needs}
				/>
			</div>
			<div>
				<label class="block text-xs font-semibold text-ink-2" for="tpl-notes">Notes</label>
				<textarea id="tpl-notes" rows="3" class="mt-1.5 w-full input" bind:value={draft.notes}></textarea>
			</div>
			<div>
				<span class="block text-xs font-semibold text-ink-2">Icon</span>
				<div class="mt-2 flex flex-wrap items-center gap-2">
					{#each sessionIcons as [icon, label] (icon)}
						<button
							type="button"
							class="flex size-10 items-center justify-center rounded-full {draft.icon === icon
								? 'bg-ink text-ground'
								: 'bg-well text-ink-2'}"
							title={label}
							aria-label={label}
							aria-pressed={draft.icon === icon}
							onclick={() => (draft.icon = draft.icon === icon ? '' : icon)}
						>
							<Icon name={icon} size="1.25rem" />
						</button>
					{/each}
				</div>
			</div>
			<div>
				<span class="block text-xs font-semibold text-ink-2">Color</span>
				<div class="mt-2 flex flex-wrap items-center gap-2.5">
					{#each presets as [color, name] (color)}
						<button
							type="button"
							class="size-8 rounded-full ring-offset-2 ring-offset-surface {draft.color === color ? 'ring-2 ring-ink' : ''}"
							style="background:{color}"
							title={name}
							aria-label={name}
							aria-pressed={draft.color === color}
							onclick={() => (draft.color = color)}
						></button>
					{/each}
					<label
						class="size-8 cursor-pointer rounded-full bg-[conic-gradient(#ef4444,#f59e0b,#10b981,#3b82f6,#8b5cf6,#ec4899,#ef4444)] ring-offset-2 ring-offset-surface {custom ? 'ring-2 ring-ink' : ''}"
						title="Custom color…"
					>
						<input type="color" class="sr-only" bind:value={draft.color} aria-label="Custom color" />
					</label>
					<button
						type="button"
						class="h-8 rounded-full px-3.5 text-xs font-bold {draft.color === ''
							? 'bg-ink text-ground'
							: 'bg-well text-ink-2'}"
						title="Remove accent color"
						aria-pressed={draft.color === ''}
						onclick={() => (draft.color = '')}>No color</button
					>
				</div>
			</div>
		</div>
	</details>

	<div class="md:grid md:grid-cols-2 md:gap-4">
		<div class="flex flex-col gap-3.5">
			{#each draft.sections as section, i (section)}
				{#snippet sectionActions()}
					<RowActions
						label={section.name || 'section'}
						index={i}
						count={draft.sections.length}
						move={(by) => moveEntry(draft.sections, i, by)}
						remove={() => removeSection(i)}
					/>
				{/snippet}
				<SectionEditor
					bind:section={draft.sections[i]}
					id="sec{i}"
					open={i === 0}
					{library}
					actions={sectionActions}
				/>
			{:else}
				<p class="px-1 text-[15px] font-semibold text-ink-2">No sections yet. Add one below.</p>
			{/each}

			<section class="rounded-3xl bg-surface p-[18px] shadow-card">
				<h3 class="text-[15px] font-bold">Add section</h3>
				<div class="mt-3 grid grid-cols-[1fr_auto] items-end gap-2">
					<div>
						<label class="block text-xs font-semibold text-ink-2" for="new-section">Name</label>
						<input
							id="new-section"
							class="mt-1.5 w-full input"
							placeholder="e.g. Warm-up, Strength, Cool-down"
							bind:value={newSection}
							onkeydown={(e) => {
								if (e.key === 'Enter') {
									e.preventDefault();
									addSection();
								}
							}}
						/>
					</div>
					<button
						type="button"
						class="flex h-12 items-center gap-1.5 rounded-full bg-well px-4 text-[15px] font-bold text-ink active:opacity-70"
						onclick={addSection}
					>
						<Icon name="plus" />
						Add section
					</button>
				</div>
			</section>
		</div>

		<!-- On a phone each row already shows its name and numbers, so the plan
		     would only repeat the editor below it. -->
		<aside class="hidden md:block">
			<TemplatePlan sections={draft.sections} />
		</aside>
	</div>

	<FormError message={error} />

	<div
		class="sticky z-20 grid grid-cols-[auto_1fr] gap-2.5 {page.data.live
			? 'bottom-[calc(8.5rem+env(safe-area-inset-bottom))]'
			: 'bottom-[calc(4.75rem+env(safe-area-inset-bottom))]'}"
	>
		<a class="flex h-14 items-center rounded-full bg-surface px-6 text-[15px] font-bold text-ink shadow-card" href={cancel}
			>Cancel</a
		>
		<Button type="submit" disabled={busy}>{busy ? 'Saving…' : 'Save'}</Button>
	</div>
</form>
