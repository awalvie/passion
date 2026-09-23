<script lang="ts">
	import { untrack } from 'svelte';
	import { describe } from '$lib/api';
	import FormError from '$lib/FormError.svelte';
	import Icon from '$lib/Icon.svelte';
	import RowActions from '$lib/RowActions.svelte';
	import SectionEditor from '$lib/SectionEditor.svelte';
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
		sources,
		save,
		cancel
	}: {
		template?: SessionTemplate;
		sources: string[];
		save: (body: ReturnType<typeof toTemplateBody>) => Promise<void>;
		cancel: string;
	} = $props();

	let draft = $state(untrack(() => toTemplateDraft(template)));
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
		try {
			await save(toTemplateBody(draft));
		} catch (e) {
			error = describe(e, templateFieldLabel);
		} finally {
			busy = false;
		}
	}
</script>

<form onsubmit={submit} oninvalidcapture={reveal}>
	<div class="space-y-4">
		<div class="card card-pad">
			<div class="flex items-start gap-2">
				{#if draft.color}
					<span class="inline-block w-3 h-3 rounded-full shrink-0 mt-2" style="background:{draft.color}"></span>
				{/if}
				<div class="min-w-0 flex-1">
					<h1 class="text-xl font-bold m-0 break-words">{draft.name || 'New session template'}</h1>
					{#if tags.length}
						<div class="mt-1 text-[11px] muted">{tags.join(' · ')}</div>
					{/if}
				</div>
				<a
					class="rounded-md btn-ghost p-2 inline-flex items-center justify-center shrink-0"
					href={cancel}
					title="Back"
					aria-label="Back"
				>
					<Icon name="arrow-left" />
				</a>
			</div>

			<details class="passion-disclosure mt-3 border-t pt-3" style="border-color:var(--border)" open={!template}>
				<summary class="text-xs font-medium muted px-1 py-1">
					<Icon name="pencil" size="0.75rem" />
					Settings
				</summary>
				<div class="mt-2 space-y-3">
					<div>
						<label class="text-xs font-medium" for="tpl-name">Name</label>
						<input id="tpl-name" class="mt-1 w-full input text-sm" maxlength="200" required bind:value={draft.name} />
					</div>
					<div>
						<label class="text-xs font-medium" for="tpl-source">Source</label>
						<input
							id="tpl-source"
							list="tpl-sources"
							class="mt-1 w-full input text-sm"
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
						<label class="text-xs font-medium" for="tpl-tags">Labels</label>
						<input
							id="tpl-tags"
							class="mt-1 w-full input text-sm"
							placeholder="comma-separated, e.g. technique, indoor"
							autocomplete="off"
							bind:value={draft.tags}
						/>
					</div>
					<div>
						<label class="text-xs font-medium" for="tpl-needs">Needs</label>
						<input
							id="tpl-needs"
							class="mt-1 w-full input text-sm"
							placeholder="equipment, e.g. hangboard, kilter, 20mm edge"
							autocomplete="off"
							bind:value={draft.needs}
						/>
					</div>
					<div>
						<label class="text-xs font-medium" for="tpl-notes">Notes</label>
						<textarea id="tpl-notes" rows="3" class="mt-1 w-full input text-sm" bind:value={draft.notes}></textarea>
					</div>
					<div>
						<span class="text-xs font-medium">Dashboard color</span>
						<div class="mt-2 flex flex-wrap items-center gap-2">
							<div class="template-color-row">
								{#each presets as [color, name] (color)}
									<button
										type="button"
										class="template-color-choice"
										class:is-selected={draft.color === color}
										style="--choice-color:{color}"
										title={name}
										aria-label={name}
										onclick={() => (draft.color = color)}
									></button>
								{/each}
								<label
									class="template-color-choice template-color-choice-custom"
									class:is-selected={custom}
									title="Custom color…"
								>
									<input type="color" class="sr-only" bind:value={draft.color} aria-label="Custom color" />
								</label>
							</div>
							<button
								type="button"
								class="template-color-none-btn rounded px-2 py-0.5 text-xs border"
								class:is-selected={draft.color === ''}
								style="border-color: var(--border)"
								title="Remove accent color"
								onclick={() => (draft.color = '')}>No color</button
							>
						</div>
					</div>
				</div>
			</details>
		</div>

		<div class="md:grid md:grid-cols-2 md:gap-4">
			<div class="space-y-3">
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
					<SectionEditor bind:section={draft.sections[i]} id="sec{i}" open={i === 0} actions={sectionActions} />
				{:else}
					<div class="card-muted p-4 text-sm muted">No sections yet. Add one below.</div>
				{/each}

				<section class="card card-pad">
					<h3 class="text-sm font-semibold">Add section</h3>
					<div class="mt-3 grid gap-2 sm:grid-cols-2 sm:items-end">
						<div>
							<label class="text-xs font-medium" for="new-section">Name</label>
							<input
								id="new-section"
								class="mt-1 w-full input"
								placeholder="e.g., Warmup, Strength, Cooldown"
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
							class="mt-1 w-full rounded-md btn-ghost px-3 py-2 text-sm font-medium"
							onclick={addSection}>+ Add section</button
						>
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
			class="lib-edit-sticky-bar m-0 rounded-lg border"
			style="border-color:var(--border)"
		>
			<button class="rounded-md btn-primary px-4 py-2 text-sm font-medium" type="submit" disabled={busy}>
				{busy ? 'Saving…' : 'Save'}
			</button>
			<span class="flex-1"></span>
			<a class="text-sm muted hover:underline" href={cancel}>Cancel</a>
		</div>
	</div>
</form>
