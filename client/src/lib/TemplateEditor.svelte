<script lang="ts">
	import { onMount, tick, untrack } from 'svelte';
	import { MediaQuery } from 'svelte/reactivity';
	import { describe, RequestFailed } from '$lib/api';
	import { kindOf, loadExercises, type Exercise } from '$lib/exercise';
	import ExerciseList from '$lib/ExerciseList.svelte';
	import FormError from '$lib/FormError.svelte';
	import NewExercise from '$lib/NewExercise.svelte';
	import RecentRuns from '$lib/RecentRuns.svelte';
	import Icon from '$lib/Icon.svelte';
	import RowActions from '$lib/RowActions.svelte';
	import SectionList from '$lib/SectionList.svelte';
	import Segmented from '$lib/Segmented.svelte';
	import SectionEditor from '$lib/SectionEditor.svelte';
	import Select from '$lib/Select.svelte';
	import Sheet from '$lib/Sheet.svelte';
	import { sessionIcons } from '$lib/SessionIcon.svelte';
	import StepEditor from '$lib/StepEditor.svelte';
	import Toast from '$lib/Toast.svelte';
	import {
		moveEntry,
		rowCounts,
		templateFieldLabel,
		toStep,
		toTemplateBody,
		toTemplateDraft,
		type Choice,
		type Section,
		type SessionTemplate,
		type Step
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

	// From lg the session sits beside a pane that shows what is picked; from xl
	// the library is a column to its left. Below lg both open as sheets.
	const wide = new MediaQuery('min-width: 64rem');
	const widest = new MediaQuery('min-width: 80rem');

	let draft = $state(untrack(() => toTemplateDraft(template)));

	// The API replaces the whole template, so nothing is kept until Save.
	const clean = JSON.stringify(draft);
	const dirty = $derived(JSON.stringify(draft) !== clean);
	let saving = false;
	guardUnsaved(() => !saving && dirty);

	// An exercise made or changed in another tab shows when the user comes back
	// to this one.
	let library = $state(untrack(() => exercises));
	async function reload() {
		try {
			library = await loadExercises(true);
		} catch {
			/* keep the list it has */
		}
	}

	type Picked = { section: Section } | { choice: Choice } | { step: Step } | null;
	type Target = { section: Section } | { choice: Choice };

	// What the pane shows; null is the session's details.
	let picked = $state<Picked>(null);
	let paneOpen = $state(false);
	function pick(p: Picked) {
		picked = p;
		paneOpen = !wide.current;
	}

	// The section a picked exercise or choice sits in.
	function home(item: Step | Choice) {
		return draft.sections.find((s) => s.items.some((i) => i.step === item || i.choice === item || i.choice?.options.includes(item as Step)));
	}
	// A removed item leaves the pane on the details.
	const shown = $derived.by((): Picked => {
		if (!picked) return null;
		if ('section' in picked) return draft.sections.includes(picked.section) ? picked : null;
		return home('step' in picked ? picked.step : picked.choice) ? picked : null;
	});
	const selected = $derived((wide.current || paneOpen) && shown && ('step' in shown ? shown.step : 'choice' in shown ? shown.choice : shown.section));

	const head = $derived.by(() => {
		if (!shown) return { eyebrow: 'Session', title: 'Details' };
		if ('section' in shown) return { eyebrow: `Section ${draft.sections.indexOf(shown.section) + 1}`, title: shown.section.name || 'Untitled section' };
		if ('choice' in shown) return { eyebrow: `In ${home(shown.choice)?.name}`, title: shown.choice.name || 'Untitled choice' };
		return { eyebrow: `In ${home(shown.step)?.name} · ${kindOf(shown.step.kind).label}`, title: shown.step.name };
	});

	// Where the library column adds, and where the Add sheet adds when open.
	let addTo = $state<Section | undefined>(untrack(() => draft.sections[0]));
	let target = $state<Target | null>(null);
	let addOpen = $state(false);
	// Each opening of the Add sheet starts with an empty search.
	let opened = $state(0);
	let search = $state<HTMLInputElement>();

	// The library lists exercises, or sections of other sessions to copy.
	let tab = $state<'exercises' | 'sections'>('exercises');

	async function find() {
		tab = 'exercises';
		await tick();
		search?.focus();
	}

	function add(to: Target) {
		if (widest.current && 'section' in to) {
			addTo = to.section;
			find();
			return;
		}
		target = to;
		tab = 'exercises';
		opened++;
		addOpen = true;
	}

	// A new exercise joins the library and goes where it was asked for.
	let making = $state<{ name: string; to: Target } | null>(null);
	let makeOpen = $state(false);
	function make(name: string, to: Target) {
		making = { name, to };
		makeOpen = true;
	}
	function made(e: Exercise) {
		library = [...library, e].sort((a, b) => a.name.localeCompare(b.name));
		if (making) put(making.to, e);
	}
	const named = (to: Target) => ('section' in to ? to.section.name : to.choice.name);

	// A copied section is the session's own: changing it leaves the other alone.
	let copied = $state<string[]>([]);
	let note = $state('');
	let undo: () => void = () => {};

	function copySection(from: Section, key: string, after: Section | undefined) {
		const at = after ? draft.sections.indexOf(after) + 1 : draft.sections.length;
		draft.sections.splice(at, 0, structuredClone($state.snapshot(from)));
		const added = draft.sections[at];
		addTo ??= added;
		copied.push(key);
		addOpen = false;
		note = `${from.name} copied`;
		undo = () => {
			const i = draft.sections.indexOf(added);
			if (i >= 0) draft.sections.splice(i, 1);
			if (addTo === added) addTo = draft.sections[0];
			const k = copied.lastIndexOf(key);
			if (k >= 0) copied.splice(k, 1);
		};
	}

	function put(to: Target, e: Exercise) {
		if ('section' in to) to.section.items.push({ step: toStep(e) });
		else to.choice.options.push(toStep(e));
	}

	function inside(to: Target | null | undefined, e: Exercise) {
		if (!to) return '';
		const steps = 'section' in to ? to.section.items.flatMap((i) => (i.step ? [i.step] : [])) : to.choice.options;
		return steps.some((s) => s.exercise === e.id) ? `In ${'section' in to ? to.section.name : to.choice.name}` : '';
	}

	// / goes to the library's search, unless the user is typing.
	function key(e: KeyboardEvent) {
		if (e.key !== '/' || !widest.current || (e.target as Element).closest('input, textarea, select, [contenteditable]')) return;
		e.preventDefault();
		find();
	}

	let nameField = $state<HTMLInputElement>();
	onMount(async () => {
		if (!rename) return;
		await showName();
		nameField?.select();
	});

	// The name sits in the details, which may be hidden behind another pane.
	async function showName() {
		picked = null;
		paneOpen = !wide.current;
		await tick();
		nameField?.focus();
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
		addTo ??= draft.sections[0];
		newSection = '';
	}

	function removeSection(i: number) {
		const n = draft.sections[i].items.length;
		if (n && !confirm('Remove this section and all its exercises?')) return;
		const [gone] = draft.sections.splice(i, 1);
		if (addTo === gone) addTo = draft.sections[0];
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
			if (e instanceof RequestFailed && e.error.fields?.name) showName();
		} finally {
			busy = false;
		}
	}
</script>

<svelte:window onfocus={reload} onkeydown={key} />

{#snippet details()}
	<div class="flex flex-col gap-3.5">
		<div>
			<label class="block text-xs font-semibold text-ink-2" for="tpl-name">Name</label>
			<input id="tpl-name" class="mt-1.5 input" maxlength="200" bind:value={draft.name} bind:this={nameField} />
		</div>
		<div>
			<label class="block text-xs font-semibold text-ink-2" for="tpl-source">Source</label>
			<input id="tpl-source" list="tpl-sources" class="mt-1.5 input" placeholder="e.g. Power Company Climbing" autocomplete="off" bind:value={draft.source} />
			<datalist id="tpl-sources">
				{#each sources as s (s)}
					<option value={s}></option>
				{/each}
			</datalist>
		</div>
		<div>
			<label class="block text-xs font-semibold text-ink-2" for="tpl-tags">Labels</label>
			<input id="tpl-tags" class="mt-1.5 input" placeholder="comma-separated, e.g. technique, indoor" autocomplete="off" bind:value={draft.tags} />
		</div>
		<div>
			<label class="block text-xs font-semibold text-ink-2" for="tpl-needs">Needs</label>
			<input id="tpl-needs" class="mt-1.5 input" placeholder="equipment, e.g. hangboard, kilter, 20mm edge" autocomplete="off" bind:value={draft.needs} />
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
						class="flex size-11 items-center justify-center rounded-full {draft.icon === icon ? 'bg-ink text-ground' : 'bg-well text-ink-2'}"
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
						class="relative size-8 rounded-full ring-offset-2 ring-offset-surface before:absolute before:-inset-1.5 before:content-[''] {draft.color === color ? 'ring-2 ring-ink' : ''}"
						style="background:{color}"
						title={name}
						aria-label={name}
						aria-pressed={draft.color === color}
						onclick={() => (draft.color = color)}
					></button>
				{/each}
				<label
					class="relative size-8 cursor-pointer rounded-full bg-[conic-gradient(#ef4444,#f59e0b,#10b981,#3b82f6,#8b5cf6,#ec4899,#ef4444)] ring-offset-2 ring-offset-surface before:absolute before:-inset-1.5 before:content-[''] {custom
						? 'ring-2 ring-ink'
						: ''}"
					title="Custom color…"
				>
					<input type="color" class="sr-only" bind:value={draft.color} aria-label="Custom color" />
				</label>
				<button
					type="button"
					class="relative h-8 rounded-full px-3.5 text-xs font-bold before:absolute before:inset-x-0 before:-inset-y-1.5 before:content-[''] {draft.color === ''
						? 'bg-ink text-ground'
						: 'bg-well text-ink-2'}"
					title="Remove accent color"
					aria-pressed={draft.color === ''}
					onclick={() => (draft.color = '')}>No color</button
				>
			</div>
		</div>
	</div>
{/snippet}

{#snippet tabs()}
	<Segmented
		label="Library"
		items={[
			{ label: 'Exercises', on: tab === 'exercises', onclick: () => (tab = 'exercises') },
			{ label: 'Sections', on: tab === 'sections', onclick: () => (tab = 'sections') }
		]}
	/>
{/snippet}

{#snippet pane()}
	{#if shown && 'step' in shown}
		{#key shown.step}
			<StepEditor bind:step={shown.step} id="pane-step" skip={wide.current ? rowCounts : []} />
			<RecentRuns exercise={shown.step.exercise} sides={shown.step.per_side ? 2 : 1} />
		{/key}
	{:else if shown && 'choice' in shown}
		<div class="flex flex-col gap-3.5">
			<div>
				<label class="block text-xs font-semibold text-ink-2" for="pane-choice">Name</label>
				<input id="pane-choice" class="mt-1.5 input" maxlength="200" bind:value={shown.choice.name} />
			</div>
			<div>
				<label class="block text-xs font-semibold text-ink-2" for="pane-choice-notes">Notes</label>
				<textarea id="pane-choice-notes" rows="3" class="mt-1.5 input" bind:value={shown.choice.notes}></textarea>
			</div>
		</div>
	{:else if shown && 'section' in shown}
		<div class="flex flex-col gap-3.5">
			<div>
				<label class="block text-xs font-semibold text-ink-2" for="pane-section">Name</label>
				<input id="pane-section" class="mt-1.5 input" maxlength="200" bind:value={shown.section.name} />
			</div>
			<div>
				<label class="block text-xs font-semibold text-ink-2" for="pane-section-notes">Notes</label>
				<textarea id="pane-section-notes" rows="3" class="mt-1.5 input" bind:value={shown.section.notes}></textarea>
			</div>
		</div>
	{:else}
		{@render details()}
	{/if}
{/snippet}

<form class="flex flex-col pb-[calc(env(safe-area-inset-bottom)+1rem)]" novalidate onsubmit={submit}>
	<div class="sticky top-0 z-30 flex flex-col gap-2 bg-ground/85 px-4 pt-[calc(env(safe-area-inset-top)+0.75rem)] pb-2 backdrop-blur-md lg:px-8 lg:pb-3 lg:shadow-[inset_0_-1px_0_var(--line)]">
		<div class="grid grid-cols-[1fr_auto_1fr] items-center gap-2">
			<a class="flex h-11 items-center justify-self-start rounded-full bg-surface px-4 text-[15px] font-bold text-ink shadow-card-sm" href={cancel} data-back>
				Cancel
			</a>
			<span class="text-[15px] font-bold">{template ? 'Edit session' : 'New session'}</span>
			<span class="flex items-center gap-4 justify-self-end">
				{#if dirty}<span class="hidden text-xs font-semibold text-ink-2 lg:inline">Unsaved changes</span>{/if}
				<button
					type="submit"
					class="flex h-11 items-center rounded-full bg-tint px-5 text-[15px] font-bold text-on-tint shadow-tint disabled:opacity-50"
					disabled={busy}
				>
					{busy ? 'Saving…' : 'Save'}
				</button>
			</span>
		</div>
		<FormError message={error} />
	</div>

	<div
		class="flex flex-col gap-3.5 px-4 pt-1.5 lg:mx-auto lg:grid lg:w-full lg:max-w-[1440px] lg:grid-cols-[minmax(0,1fr)_360px] lg:items-start lg:gap-5 lg:px-8 lg:pt-5 xl:grid-cols-[320px_minmax(0,1fr)_360px]"
	>
		{#if widest.current}
			<aside class="sticky top-[5.5rem] flex max-h-[calc(100dvh-7rem)] flex-col gap-3 overflow-y-auto overscroll-contain rounded-3xl bg-surface p-4 shadow-card" aria-label="Library">
				{@render tabs()}
				{#if tab === 'sections'}
					<SectionList exclude={template?.id} {copied} copy={(from, key) => copySection(from, key, addTo)} />
				{:else if draft.sections.length && addTo}
					<ExerciseList
						exercises={library}
						bind:input={search}
						pick={(e) => put({ section: addTo! }, e)}
						inside={(e) => inside({ section: addTo! }, e)}
						onnew={(name) => make(name, { section: addTo! })}
					>
						<label class="flex items-center gap-3 text-xs font-semibold text-ink-2">
							Add to
							<span class="min-w-0 flex-1">
								<Select
									label="Add to"
									bind:value={() => String(draft.sections.indexOf(addTo!)), (v) => (addTo = draft.sections[Number(v)])}
									options={draft.sections.map((s, i) => ({ value: String(i), label: `${i + 1} · ${s.name || 'Untitled section'}` }))}
								/>
							</span>
						</label>
					</ExerciseList>
				{:else}
					<p class="px-1 py-2 text-[15px] font-semibold text-ink-2">Add a section, then add exercises to it from here.</p>
				{/if}
			</aside>
		{/if}

		<div class="flex min-w-0 flex-col gap-3.5">
			<button type="button" class="flex min-h-20 items-center gap-3.5 rounded-3xl bg-surface px-4 py-3.5 text-left shadow-card lg:hidden" onclick={() => pick(null)}>
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
					<span class="block text-xl leading-tight font-extrabold tracking-tight break-words">{draft.name || 'New session'}</span>
					<span class="mt-0.5 block text-xs font-semibold text-ink-2">{meta || 'Details, icon and color'}</span>
				</span>
				<span class="text-ink-3"><Icon name="chevron-right" /></span>
			</button>

			{#each draft.sections as section, i (section)}
				{#snippet sectionActions()}
					<RowActions
						label={section.name || 'section'}
						index={i}
						count={draft.sections.length}
						move={(by) => moveEntry(draft.sections, i, by)}
						remove={() => removeSection(i)}
						more={[{ label: 'Name and notes', onclick: () => pick({ section }) }]}
					/>
				{/snippet}
				<SectionEditor
					bind:section={draft.sections[i]}
					id="sec{i}"
					number={i + 1}
					open={i === 0}
					target={widest.current && section === addTo}
					{selected}
					onselect={pick}
					onadd={add}
					actions={sectionActions}
				/>
			{:else}
				<p class="px-1 text-[15px] font-semibold text-ink-2">No sections yet. Add one below.</p>
			{/each}

			<details class="rounded-3xl bg-surface px-4 shadow-card" open={!draft.sections.length}>
				<summary class="flex min-h-14 cursor-pointer list-none items-center gap-2.5 text-[15px] font-bold [&::-webkit-details-marker]:hidden">
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
					<button type="button" class="flex h-12 items-center gap-1.5 rounded-full bg-well px-4 text-[15px] font-bold text-ink active:opacity-70" onclick={addSection}>
						Add
					</button>
				</div>
			</details>
		</div>

		{#if wide.current}
			<aside class="sticky top-[5.5rem] flex max-h-[calc(100dvh-7rem)] flex-col gap-4 overflow-y-auto overscroll-contain rounded-3xl bg-surface p-5 shadow-card" aria-label={head.title}>
				{#if shown}
					<button type="button" class="-mt-1 flex h-11 items-center gap-2 self-start text-[15px] font-bold text-ink-2" onclick={() => (picked = null)}>
						<Icon name="chevron-left" />Session details
					</button>
				{/if}
				<div>
					<p class="text-xs font-semibold text-ink-2">{head.eyebrow}</p>
					<h2 class="text-xl leading-tight font-extrabold tracking-tight break-words">{head.title}</h2>
				</div>
				{@render pane()}
			</aside>
		{/if}
	</div>
</form>

{#if !wide.current}
	<Sheet bind:open={paneOpen} eyebrow={head.eyebrow} title={head.title}>
		{@render pane()}
	</Sheet>
{/if}

<Sheet
	bind:open={addOpen}
	eyebrow={target && 'section' in target ? `Section ${draft.sections.indexOf(target.section) + 1}` : 'Choice'}
	title="Add to {target ? ('section' in target ? target.section.name : target.choice.name) : ''}"
>
	{#if target}
		{@const to = target}
		{#key opened}
		<div class="flex flex-col gap-3">
			{#if 'section' in to}{@render tabs()}{/if}
			{#if tab === 'sections' && 'section' in to}
				<SectionList exclude={template?.id} {copied} copy={(from, key) => copySection(from, key, to.section)} />
			{:else}
				<ExerciseList exercises={library} pick={(e) => put(to, e)} inside={(e) => inside(to, e)} onnew={(name) => make(name, to)} />
			{/if}
		</div>
		{/key}
	{/if}
</Sheet>

<NewExercise bind:open={makeOpen} name={making?.name ?? ''} where={making ? named(making.to) : ''} {sources} onsaved={made} />

<div class="[--above-bar:calc(1rem+env(safe-area-inset-bottom))]">
	<Toast bind:message={note} action="Undo" onaction={() => undo()} />
</div>
