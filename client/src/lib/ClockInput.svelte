<script lang="ts">
	let {
		seconds = $bindable(),
		id,
		autofocus = false
	}: { seconds: number | null; id: string; autofocus?: boolean } = $props();

	// Digits fill from the right, as on a microwave: 340 is 3:40. The last two
	// are seconds, so 90 stays 0:90 as typed rather than jumping to 1:30.
	const start = seconds;
	let digits = $state(start == null ? '' : `${Math.floor(start / 60) || ''}${String(start % 60).padStart(2, '0')}`.replace(/^0+/, ''));
	const shown = $derived(digits ? `${digits.slice(0, -2) || '0'}:${digits.slice(-2).padStart(2, '0')}` : '');

	function input(e: Event & { currentTarget: HTMLInputElement }) {
		digits = e.currentTarget.value.replace(/\D/g, '').replace(/^0+/, '').slice(0, 4);
		e.currentTarget.value = shown;
		seconds = digits ? Number(digits.slice(0, -2) || 0) * 60 + Number(digits.slice(-2)) : null;
	}
</script>

<!-- svelte-ignore a11y_autofocus -->
<input {id} class="input" type="text" inputmode="numeric" autocomplete="off" placeholder="0:00" value={shown} oninput={input} {autofocus} />
