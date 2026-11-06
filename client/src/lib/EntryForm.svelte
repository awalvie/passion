<script lang="ts">
	import { onDestroy, onMount, untrack } from 'svelte';
	import { describe, request } from './api';
	import DateField from './DateField.svelte';
	import FormBar from './FormBar.svelte';
	import { haptic } from './haptics';
	import Icon from './Icon.svelte';
	import { newId } from './id';
	import type { JournalEntry } from './journal';
	import { notice } from './notice.svelte';
	import { guardUnsaved } from './unsaved';

	// New entry when there is no entry, else Edit entry. Pin is only on a new
	// one; an entry already made pins from its own menu. Beside the Journal's
	// list, its title is an h2 under the page's h1.
	let {
		entry,
		today,
		cancel,
		saved,
		heading = 'h1'
	}: {
		entry?: JournalEntry;
		today: string;
		cancel: string;
		saved: (entry: JournalEntry) => Promise<unknown>;
		heading?: 'h1' | 'h2';
	} = $props();

	type Draft = { id: string; local_date: string; body: string; pin: boolean; base: string };
	const first = untrack(() => entry);
	const key = `passion-journal-draft:${first?.id ?? 'new'}`;

	// What was typed and not saved stays on the phone, so a phone that closes
	// the app loses nothing. An edit keeps its draft only while the entry has
	// not changed elsewhere.
	const kept = (() => {
		try {
			const d: Draft | null = JSON.parse(localStorage.getItem(key) ?? 'null');
			return d && d.base === (first?.body ?? '') ? d : null;
		} catch {
			return null;
		}
	})();

	const id = first?.id ?? kept?.id ?? newId();
	const base = { local_date: first?.local_date ?? untrack(() => today), body: first?.body ?? '' };
	let date = $state(kept?.local_date ?? base.local_date);
	let body = $state(kept?.body ?? base.body);
	let pin = $state(kept?.pin ?? false);

	let busy = $state(false);
	let error = $state('');
	let saving = false;

	const changed = $derived(body.trim() !== base.body.trim() || date !== base.local_date || pin);
	const ready = $derived(!!body.trim() && changed && !busy);
	guardUnsaved(() => !saving && changed);

	$effect(() => {
		const d: Draft = { id, local_date: date, body, pin, base: base.body };
		try {
			if (changed) localStorage.setItem(key, JSON.stringify(d));
			else localStorage.removeItem(key);
		} catch {
			/* only this page holds it */
		}
	});

	// Leaving by Save or by a confirmed Cancel ends the draft. A closed app
	// never gets here, so its draft stays.
	onDestroy(() => {
		try {
			localStorage.removeItem(key);
		} catch {
			/* nothing kept */
		}
	});

	let box = $state<HTMLTextAreaElement>();
	onMount(() => {
		if (!first) box?.focus();
	});

	async function submit(e: SubmitEvent) {
		e.preventDefault();
		if (!ready) return;
		busy = true;
		error = '';
		try {
			let out = await request<JournalEntry>('PUT', `/api/v1/journal/${id}`, { local_date: date, body: body.trim() });
			if (pin) out = await request<JournalEntry>('PUT', `/api/v1/journal/${id}/pin`);
			saving = true;
			await saved(out);
			notice.text = 'Entry saved';
		} catch (err) {
			saving = false;
			error = describe(err, (f) => ({ local_date: 'Date', body: 'Entry' })[f] ?? f);
		} finally {
			busy = false;
		}
	}
</script>

<form class="flex flex-col gap-3.5 px-4 pb-[calc(env(safe-area-inset-bottom)+1.5rem)]" onsubmit={submit}>
	<FormBar title={entry ? 'Edit entry' : 'New entry'} {cancel} {heading} {busy} disabled={!ready} {error} class="-mx-4" />

	<div class="flex flex-col rounded-3xl bg-surface shadow-card">
		<textarea
			bind:this={box}
			bind:value={body}
			aria-label="Entry"
			placeholder="What did you notice?"
			class="min-h-36 resize-none bg-transparent p-[18px] pb-2 text-[15px] font-semibold [field-sizing:content] text-ink outline-none placeholder:text-ink-3"
		></textarea>
		<div class="flex items-center justify-between gap-2 px-[18px] pb-[18px]">
			<DateField bind:value={date} label="Date" max={today} {today} pill />
			{#if !entry}
				<button
					type="button"
					aria-pressed={pin}
					aria-describedby="pin-note"
					onclick={() => (pin = !pin)}
					use:haptic
					class="flex h-11 items-center gap-2 rounded-full bg-well px-4 text-[15px] font-bold text-ink focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ink {pin
						? 'ring-2 ring-ink ring-inset'
						: ''}"
				>
					<Icon name="pin" size="1rem" />{pin ? 'Pinned' : 'Pin'}
				</button>
			{/if}
		</div>
	</div>
	<!-- Read with the button always; shown once Pin is on. -->
	{#if !entry}
		<p id="pin-note" class={pin ? '-mt-1 px-1 text-xs font-semibold text-ink-2' : 'sr-only'}>Shows on Today and at the start of each session.</p>
	{/if}
</form>
