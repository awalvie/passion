<script lang="ts">
	import { tick, untrack } from 'svelte';
	import Checkbox from './Checkbox.svelte';
	import { openRun } from './runState.svelte';
	import Sheet from './Sheet.svelte';

	// Write in journal, from a running session. Like the Note sheet it saves on
	// its way out, and a second opening shows the entry already written.
	let { open = $bindable(false) }: { open?: boolean } = $props();

	let body = $state('');
	let pin = $state(false);
	let box = $state<HTMLTextAreaElement>();

	// Each opening starts from the entry as saved, with the cursor at its end.
	$effect(() => {
		if (!open) return;
		untrack(() => {
			body = openRun.journal?.body ?? '';
			pin = openRun.journal?.pin ?? false;
		});
		void tick().then(() => {
			box?.focus();
			box?.setSelectionRange(box.value.length, box.value.length);
		});
	});

	// Cleared text keeps the entry as it was; Delete is on the entry in the
	// Journal.
	function save() {
		const text = body.trim();
		const was = openRun.journal;
		if (!text || (text === was?.body && pin === was.pin)) return;
		openRun.writeJournal(text, pin);
	}

	$effect(() => {
		if (!open) untrack(save);
	});
</script>

<Sheet bind:open title={openRun.journal ? 'Edit entry' : 'New entry'} eyebrow="Journal · {openRun.run!.name}">
	<textarea
		bind:this={box}
		bind:value={body}
		onchange={save}
		onblur={save}
		aria-label="Entry"
		placeholder="What did you notice?"
		class="min-h-36 w-full resize-none rounded-3xl bg-surface p-4 text-[15px] font-semibold text-ink outline-none placeholder:text-ink-3"
	></textarea>
	<label class="mt-3 flex cursor-pointer items-start gap-3 rounded-3xl bg-surface px-4 py-3.5">
		<Checkbox bind:checked={pin} onchange={save} />
		<span>
			<span class="block text-[15px] font-bold">Pin</span>
			<span class="mt-0.5 block text-xs font-semibold text-ink-2">Shows on Today and at the start of each session.</span>
		</span>
	</label>
</Sheet>
