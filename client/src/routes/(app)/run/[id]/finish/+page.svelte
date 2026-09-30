<script lang="ts">
	import { goto } from '$app/navigation';
	import { describe, request } from '$lib/api';
	import Button from '$lib/Button.svelte';
	import FormError from '$lib/FormError.svelte';
	import NavBar from '$lib/NavBar.svelte';
	import { secondsSince, type Run } from '$lib/run';
	import { openRun } from '$lib/runState.svelte';

	const run = $derived(openRun.run!);

	// The journal is optional, so each field starts from what the run holds.
	let sleep = $state(openRun.run!.sleep);
	let energy = $state(openRun.run!.energy);
	let rpe = $state(openRun.run!.rpe);
	let focus = $state(openRun.run!.focus ?? '');
	let setting = $state(openRun.run!.setting ?? '');
	let wentWell = $state(openRun.run!.went_well ?? '');
	let nextFocus = $state(openRun.run!.next_focus ?? '');
	let notes = $state(openRun.run!.notes ?? '');

	let busy = $state(false);
	let error = $state('');

	const focuses = ['strength', 'endurance', 'technique', 'projects', 'general'];

	async function finish() {
		busy = true;
		error = '';
		const r = openRun.run!;
		openRun.logTimer();
		Object.assign(r, {
			sleep,
			energy,
			rpe,
			focus: focus || null,
			setting: setting || null,
			went_well: wentWell.trim() || null,
			next_focus: nextFocus.trim() || null,
			notes: notes.trim() || null,
			elapsed_seconds: r.elapsed_seconds ?? secondsSince(r.started_at)
		});
		openRun.saveBody();
		try {
			if (!(await openRun.settle())) {
				error = 'Not saved yet. Finish needs a signal. It is kept on this phone, so try again.';
				return;
			}
			await request<Run>('POST', `/api/v1/runs/${r.id}/finish`);
			await goto(`/history/${r.id}`);
			openRun.forget();
		} catch (e) {
			error = describe(e, (f) => f);
		} finally {
			busy = false;
		}
	}
</script>

{#snippet rating(label: string, max: number, value: number | null, set: (v: number | null) => void)}
	<fieldset class="flex flex-col gap-2">
		<legend class="px-1 pb-2 text-sm text-ink-2">{label}</legend>
		<div class="flex gap-1 rounded-xl bg-surface p-1 shadow-sm">
			{#each Array.from({ length: max }, (_, i) => i + 1) as n (n)}
				<button
					type="button"
					class="h-10 flex-1 rounded-lg text-base {value === n ? 'bg-tint font-semibold text-on-tint' : 'text-ink'}"
					aria-pressed={value === n}
					onclick={() => set(value === n ? null : n)}
				>
					{n}
				</button>
			{/each}
		</div>
	</fieldset>
{/snippet}

<svelte:head><title>Finish {run.name}</title></svelte:head>

<NavBar title="Finish" back={{ href: `/run/${run.id}`, label: 'Session' }} />

<div class="flex flex-col gap-5 px-4 pt-2 pb-[calc(env(safe-area-inset-bottom)+2rem)]">
	<p class="text-base text-ink-2">All optional. It helps to read it before the next session.</p>

	{@render rating('Sleep', 5, sleep, (v) => (sleep = v))}
	{@render rating('Energy before', 5, energy, (v) => (energy = v))}
	{@render rating('How hard it felt', 10, rpe, (v) => (rpe = v))}

	<div class="grid grid-cols-2 gap-3">
		<label class="flex flex-col gap-1 text-sm text-ink-2">
			Focus
			<select class="input" bind:value={focus}>
				<option value="">–</option>
				{#each focuses as f (f)}
					<option value={f}>{f[0].toUpperCase() + f.slice(1)}</option>
				{/each}
			</select>
		</label>
		<label class="flex flex-col gap-1 text-sm text-ink-2">
			Where
			<select class="input" bind:value={setting}>
				<option value="">–</option>
				<option value="indoor">Indoor</option>
				<option value="outdoor">Outdoor</option>
			</select>
		</label>
	</div>

	<label class="flex flex-col gap-1 text-sm text-ink-2">
		What went well
		<textarea class="min-h-20 rounded-2xl bg-surface p-3 text-base text-ink shadow-sm outline-none" bind:value={wentWell}></textarea>
	</label>
	<label class="flex flex-col gap-1 text-sm text-ink-2">
		Next time, focus on
		<textarea class="min-h-20 rounded-2xl bg-surface p-3 text-base text-ink shadow-sm outline-none" bind:value={nextFocus}></textarea>
	</label>
	<label class="flex flex-col gap-1 text-sm text-ink-2">
		Notes
		<textarea class="min-h-20 rounded-2xl bg-surface p-3 text-base text-ink shadow-sm outline-none" bind:value={notes}></textarea>
	</label>

	<FormError message={error} />
	<Button disabled={busy} onclick={finish}>Finish session</Button>
</div>
