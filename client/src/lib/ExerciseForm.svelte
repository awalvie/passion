<script lang="ts">
	import DurationInput from '$lib/DurationInput.svelte';
	import Icon from '$lib/Icon.svelte';
	import { allCounts, countLabels, kindOf, kinds, type Count, type Draft } from '$lib/exercise';

	let {
		draft = $bindable(),
		shown,
		sources,
		disabled = false
	}: { draft: Draft; shown: (c: Count) => boolean; sources: string[]; disabled?: boolean } = $props();

	const counts = $derived(allCounts.filter((c) => c !== 'duration_seconds' && shown(c)));
</script>

<fieldset {disabled} class="min-w-0">
	<div class="grid gap-3.5 sm:grid-cols-2">
		<div>
			<label class="block text-xs font-semibold text-ink-2" for="ex-name">Name</label>
			<input id="ex-name" class="mt-1.5 w-full input" maxlength="200" bind:value={draft.name} required />
		</div>
		<div>
			<label class="block text-xs font-semibold text-ink-2" for="ex-kind">Type</label>
			<select id="ex-kind" class="mt-1.5 w-full input" bind:value={draft.kind}>
				{#each kinds as k (k.kind)}
					<option value={k.kind}>{k.label}</option>
				{/each}
			</select>
			<p class="mt-1 mb-0 text-xs font-semibold text-ink-3">{kindOf(draft.kind).hint}</p>
		</div>
		<div>
			<label class="block text-xs font-semibold text-ink-2" for="ex-source">Source</label>
			<input
				id="ex-source"
				list="ex-sources"
				class="mt-1.5 w-full input"
				placeholder="e.g. Power Company Climbing"
				autocomplete="off"
				bind:value={draft.source}
			/>
			<datalist id="ex-sources">
				{#each sources as s (s)}
					<option value={s}></option>
				{/each}
			</datalist>
		</div>
		<div>
			<label class="block text-xs font-semibold text-ink-2" for="ex-tags">Labels</label>
			<input
				id="ex-tags"
				class="mt-1.5 w-full input"
				placeholder="comma-separated, e.g. technique, fingers"
				autocomplete="off"
				bind:value={draft.tags}
			/>
		</div>
	</div>

	{#if counts.length}
		<div class="mt-[18px] pt-4 shadow-[inset_0_1px_0_var(--line)]">
			<div class="mb-2.5 text-xs font-bold tracking-wider text-ink-3 uppercase">Configuration</div>
			<div class="grid grid-cols-3 gap-2.5">
				{#each counts as c (c)}
					<div>
						<label class="block text-xs font-semibold text-ink-2" for="ex-{c}">{countLabels[c]}</label>
						<input
							id="ex-{c}"
							type="number"
							min="0"
							step="1"
							class="mt-1.5 w-full input text-center"
							bind:value={draft[c]}
						/>
					</div>
				{/each}
			</div>
		</div>
	{/if}

	{#if shown('duration_seconds')}
		<div class="mt-[18px] pt-4 shadow-[inset_0_1px_0_var(--line)]">
			<div class="mb-2.5 text-xs font-bold tracking-wider text-ink-3 uppercase">Duration</div>
			<DurationInput bind:seconds={draft.duration_seconds} id="ex" />
		</div>
	{/if}

	{#if allCounts.some(shown)}
		<div class="mt-3">
			<label class="flex min-h-10 cursor-pointer items-center gap-2.5 text-[15px] font-semibold">
				<input type="checkbox" class="size-5 accent-ink" bind:checked={draft.per_side} />
				Per side
			</label>
			<p class="mb-0 text-xs font-semibold text-ink-3">The numbers are for each side, as in 6 reps per side.</p>
		</div>
	{/if}

	<div class="mt-[18px] pt-4 shadow-[inset_0_1px_0_var(--line)]">
		<label class="block text-xs font-semibold text-ink-2" for="ex-notes">Notes</label>
		<textarea id="ex-notes" class="mt-1.5 w-full input" rows="4" bind:value={draft.notes}></textarea>
	</div>

	<div class="mt-[18px] pt-4 shadow-[inset_0_1px_0_var(--line)]">
		<span class="block text-xs font-semibold text-ink-2">Media</span>
		{#if draft.media.length}
			<div class="mt-1.5 flex flex-col gap-2.5">
				{#each draft.media as m, i (m)}
					<div class="grid grid-cols-2 gap-2">
						<input
							class="w-full input"
							placeholder="Thumbnail URL (https://…)"
							aria-label="Thumbnail URL {i + 1}"
							bind:value={m.thumb_url}
						/>
						<div class="flex items-center gap-1.5">
							<input
								class="w-full input"
								placeholder="Video URL"
								aria-label="Video URL {i + 1}"
								bind:value={m.url}
							/>
							{#if !disabled}
								<button
									type="button"
									class="flex size-10 shrink-0 items-center justify-center rounded-full bg-well text-xl leading-none text-ink-2 active:opacity-70"
									title="Remove"
									aria-label="Remove media row {i + 1}"
									onclick={() => draft.media.splice(i, 1)}>&times;</button
								>
							{/if}
						</div>
					</div>
				{/each}
			</div>
		{:else if disabled}
			<p class="mt-1 text-[15px] font-semibold text-ink-2">No media added</p>
		{/if}
		{#if !disabled}
			<button
				type="button"
				class="mt-2.5 flex h-10 items-center gap-1.5 rounded-full bg-well px-4 text-[15px] font-bold text-ink active:opacity-70"
				onclick={() => draft.media.push({ url: '', thumb_url: '' })}
			>
				<Icon name="plus" />
				Add media
			</button>
		{/if}
	</div>
</fieldset>

<style>
	/* A locked exercise reads as a record, not a form waiting for input. */
	fieldset:disabled .input {
		background: transparent;
		box-shadow: inset 0 0 0 1px var(--line);
		color: var(--ink);
		-webkit-text-fill-color: var(--ink);
		cursor: default;
		opacity: 1;
	}
	fieldset:disabled .input::placeholder {
		color: transparent;
	}
</style>
