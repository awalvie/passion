<script lang="ts">
	import DurationInput from '$lib/DurationInput.svelte';
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
	<div class="grid gap-4 sm:grid-cols-2">
		<div>
			<label class="text-sm font-medium" for="ex-name">Name</label>
			<input id="ex-name" class="mt-1 w-full input" maxlength="200" bind:value={draft.name} required />
		</div>
		<div>
			<label class="text-sm font-medium" for="ex-kind">Type</label>
			<select id="ex-kind" class="mt-1 w-full input text-sm" bind:value={draft.kind}>
				{#each kinds as k (k.kind)}
					<option value={k.kind}>{k.label}</option>
				{/each}
			</select>
			<p class="text-[10px] muted mt-0.5 mb-0">{kindOf(draft.kind).hint}</p>
		</div>
		<div>
			<label class="text-sm font-medium" for="ex-source">Source</label>
			<input
				id="ex-source"
				list="ex-sources"
				class="mt-1 w-full input"
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
			<label class="text-sm font-medium" for="ex-tags">Labels</label>
			<input
				id="ex-tags"
				class="mt-1 w-full input"
				placeholder="comma-separated, e.g. technique, fingers"
				autocomplete="off"
				bind:value={draft.tags}
			/>
		</div>
	</div>

	{#if counts.length}
		<div class="mt-5 pt-5 divider">
			<div class="text-xs font-semibold muted uppercase tracking-widest mb-3">Configuration</div>
			<div class="grid gap-2 grid-cols-3">
				{#each counts as c (c)}
					<div>
						<label class="text-xs font-medium" for="ex-{c}">{countLabels[c]}</label>
						<input
							id="ex-{c}"
							type="number"
							min="0"
							step="1"
							class="mt-1 w-full input"
							bind:value={draft[c]}
						/>
					</div>
				{/each}
			</div>
		</div>
	{/if}

	{#if shown('duration_seconds')}
		<div class="mt-5 pt-5 divider">
			<div class="text-xs font-semibold muted uppercase tracking-widest mb-3">Duration</div>
			<DurationInput bind:seconds={draft.duration_seconds} id="ex" />
		</div>
	{/if}

	<div class="mt-5 pt-5 divider">
		<label class="text-sm font-medium" for="ex-notes">Notes</label>
		<textarea id="ex-notes" class="mt-1 w-full input" rows="4" bind:value={draft.notes}></textarea>
	</div>

	<div class="mt-5 pt-5 divider">
		<span class="block text-sm font-medium">Media</span>
		{#if draft.media.length}
			<div class="space-y-2 mt-2">
				{#each draft.media as m, i (m)}
					<div class="grid grid-cols-2 gap-2">
						<input
							class="w-full input text-sm"
							placeholder="Thumbnail URL (https://…)"
							aria-label="Thumbnail URL {i + 1}"
							bind:value={m.thumb_url}
						/>
						<div class="flex gap-1">
							<input
								class="w-full input text-sm"
								placeholder="Video URL"
								aria-label="Video URL {i + 1}"
								bind:value={m.url}
							/>
							{#if !disabled}
								<button
									type="button"
									class="btn-ghost px-2 text-xs rounded-md"
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
			<p class="mt-1 text-sm muted">No media added</p>
		{/if}
		{#if !disabled}
			<button
				type="button"
				class="mt-2 text-xs btn-ghost px-2 py-1 rounded-md"
				onclick={() => draft.media.push({ url: '', thumb_url: '' })}>+ Add media</button
			>
		{/if}
	</div>
</fieldset>

<style>
	/* A locked exercise reads as a record, not a form waiting for input. */
	fieldset:disabled .input {
		background: var(--card-muted);
		cursor: default;
	}
	fieldset:disabled .input::placeholder {
		color: transparent;
	}
</style>
