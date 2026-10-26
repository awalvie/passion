<script lang="ts">
	import { onMount, untrack } from 'svelte';
	import { describe } from '$lib/api';
	import { loadExercises, type Exercise } from '$lib/exercise';
	import FormError from '$lib/FormError.svelte';
	import Icon from '$lib/Icon.svelte';
	import RowActions from '$lib/RowActions.svelte';
	import SectionEditor from '$lib/SectionEditor.svelte';
	import { sessionIcons } from '$lib/SessionIcon.svelte';
	import {
		moveEntry,
		templateFieldLabel,
		toTemplateBody,
		toTemplateDraft,
		type SessionTemplate
	} from '$lib/template';
	import { guardUnsaved } from '$lib/unsaved';

	let {
		template,
		exercises,
		sources,
		save,
		cancel,
		rename = false
	}: {
		template?: SessionTemplate;
		exercises: Exercise[];
		sources: string[];
		save: (body: ReturnType<typeof toTemplateBody>) => Promise<void>;
		cancel: string;
		// A copy just made opens with its name picked out to type over.
		rename?: boolean;
	} = $props();

	let nameField = $state<HTMLInputElement>();
	onMount(() => {
		if (!rename) return;
		nameField?.focus();
		nameField?.select();
	});

	let draft = $state(untrack(() => toTemplateDraft(template)));

	// The API replaces the whole template, so nothing is kept until Save.
	const clean = JSON.stringify(draft);
	let saving = false;
	guardUnsaved(() => !saving && JSON.stringify(draft) !== clean);

	// A missing exercise is made in another tab, so the list reloads when the
	// user comes back to this one.
	let library = $state(untrack(() => exercises));
	async function reload() {
		try {
			library = await loadExercises(true);
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
	const meta = $derived([...tags, draft.needs.trim()].filter(Boolean).join(' · '));
	const icon = $derived(sessionIcons.find(([name]) => name === draft.icon)?.[0]);
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

<form
	class="flex flex-col gap-3.5 px-4 pb-[calc(env(safe-area-inset-bottom)+1rem)]"
	onsubmit={submit}
	oninvalidcapture={reveal}
>
	<div class="sticky top-0 z-30 -mx-4 flex flex-col gap-2 bg-ground/85 px-4 pt-[calc(env(safe-area-inset-top)+0.75rem)] pb-2 backdrop-blur-md">
		<div class="grid grid-cols-[1fr_auto_1fr] items-center gap-2">
			<a
				class="flex h-11 items-center justify-self-start rounded-full bg-surface px-4 text-[15px] font-bold text-ink shadow-card-sm"
				href={cancel}
				data-back
			>
				Cancel
			</a>
			<span class="text-[15px] font-bold">{template ? 'Edit session' : 'New session'}</span>
			<button
				type="submit"
				class="flex h-11 items-center justify-self-end rounded-full bg-tint px-5 text-[15px] font-bold text-on-tint shadow-tint disabled:opacity-50"
				disabled={busy}
			>
				{busy ? 'Saving…' : 'Save'}
			</button>
		</div>
		<FormError message={error} />
	</div>

	<details class="group/settings rounded-3xl bg-surface shadow-card" open={!template || rename}>
		<summary
			class="flex min-h-20 cursor-pointer list-none items-center gap-3.5 px-4 py-3.5 [&::-webkit-details-marker]:hidden"
		>
			<span
				class="flex size-12 shrink-0 items-center justify-center rounded-[15px] text-xl font-bold text-ink {draft.color ? '' : 'bg-well'}"
				style={draft.color ? `background: color-mix(in oklab, ${draft.color} 22%, var(--surface))` : ''}
				aria-hidden="true"
			>
				{#if icon}
					<Icon name={icon} size="1.5rem" />
				{:else}
					{(draft.name.trim() || 'N').charAt(0).toUpperCase()}
				{/if}
			</span>
			<span class="min-w-0 flex-1">
				<h1 class="text-xl leading-tight font-extrabold tracking-tight break-words">{draft.name || 'New session'}</h1>
				{#if meta}
					<span class="mt-0.5 block text-xs font-semibold text-ink-2">{meta}</span>
				{/if}
			</span>
			<span class="text-ink-3"><Icon name="pencil" /></span>
		</summary>
		<div class="flex flex-col gap-3.5 px-[18px] pt-4 pb-[18px] shadow-[inset_0_1px_0_var(--line)]">
			<div>
				<label class="block text-xs font-semibold text-ink-2" for="tpl-name">Name</label>
				<input id="tpl-name" class="mt-1.5 input" maxlength="200" required bind:value={draft.name} bind:this={nameField} />
			</div>
			<div>
				<label class="block text-xs font-semibold text-ink-2" for="tpl-source">Source</label>
				<input
					id="tpl-source"
					list="tpl-sources"
					class="mt-1.5 input"
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
					class="mt-1.5 input"
					placeholder="comma-separated, e.g. technique, indoor"
					autocomplete="off"
					bind:value={draft.tags}
				/>
			</div>
			<div>
				<label class="block text-xs font-semibold text-ink-2" for="tpl-needs">Needs</label>
				<input
					id="tpl-needs"
					class="mt-1.5 input"
					placeholder="equipment, e.g. hangboard, kilter, 20mm edge"
					autocomplete="off"
					bind:value={draft.needs}
				/>
			</div>
			<div>
				<label class="block text-xs font-semibold text-ink-2" for="tpl-notes">Notes</label>
				<textarea id="tpl-notes" rows="3" class="mt-1.5 input" bind:value={draft.notes}></textarea>
			</div>
			<div>
				<span class="block text-xs font-semibold text-ink-2">Icon</span>
				<div class="mt-2 flex flex-wrap items-center gap-2">
					{#each sessionIcons as [icon, label] (icon)}
						<button
							type="button"
							class="flex size-11 items-center justify-center rounded-full {draft.icon === icon
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
				<div class="mt-2 flex flex-wrap items-center gap-3">
					{#each presets as [color, name] (color)}
						<button
							type="button"
							class="relative before:absolute before:-inset-1.5 before:content-[''] size-8 rounded-full ring-offset-2 ring-offset-surface {draft.color === color ? 'ring-2 ring-ink' : ''}"
							style="background:{color}"
							title={name}
							aria-label={name}
							aria-pressed={draft.color === color}
							onclick={() => (draft.color = color)}
						></button>
					{/each}
					<label
						class="relative before:absolute before:-inset-1.5 before:content-[''] size-8 cursor-pointer rounded-full bg-[conic-gradient(#ef4444,#f59e0b,#10b981,#3b82f6,#8b5cf6,#ec4899,#ef4444)] ring-offset-2 ring-offset-surface {custom ? 'ring-2 ring-ink' : ''}"
						title="Custom color…"
					>
						<input type="color" class="sr-only" bind:value={draft.color} aria-label="Custom color" />
					</label>
					<button
						type="button"
						class="relative h-8 rounded-full px-3.5 text-xs font-bold before:absolute before:-inset-y-1.5 before:inset-x-0 before:content-[''] {draft.color === ''
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
				number={i + 1}
				open={i === 0}
				{library}
				actions={sectionActions}
			/>
		{:else}
			<p class="px-1 text-[15px] font-semibold text-ink-2">No sections yet. Add one below.</p>
		{/each}

		<details class="rounded-3xl bg-surface px-4 shadow-card" open={!draft.sections.length}>
			<summary
				class="flex min-h-14 cursor-pointer list-none items-center gap-2.5 text-[15px] font-bold [&::-webkit-details-marker]:hidden"
			>
				<Icon name="plus" />Add section
			</summary>
			<div class="grid grid-cols-[1fr_auto] items-end gap-2 pb-4">
				<div>
					<label class="block text-xs font-semibold text-ink-2" for="new-section">Name</label>
					<input
						id="new-section"
						class="mt-1.5 input"
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
					Add
				</button>
			</div>
		</details>
	</div>
</form>
