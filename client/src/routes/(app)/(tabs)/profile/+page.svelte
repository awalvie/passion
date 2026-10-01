<script lang="ts">
	import Segmented from '$lib/Segmented.svelte';
	import { goto } from '$app/navigation';
	import { request } from '$lib/api';
	import { setSound, soundOn } from '$lib/audio';
	import Button from '$lib/Button.svelte';
	import NavBar from '$lib/NavBar.svelte';
	import { hapticsOn, setHaptics, tickBox } from '$lib/haptics';
	import { clearRunStorage, clearToken } from '$lib/session';

	type Theme = 'system' | 'light' | 'dark';
	const themes: { value: Theme; label: string }[] = [
		{ value: 'system', label: 'System' },
		{ value: 'light', label: 'Light' },
		{ value: 'dark', label: 'Dark' }
	];

	let sound = $state(soundOn());
	let haptics = $state(hapticsOn());

	let theme = $state((localStorage.getItem('passion-theme') as Theme | null) ?? 'system');

	function setTheme(next: Theme) {
		theme = next;
		if (next === 'system') localStorage.removeItem('passion-theme');
		else localStorage.setItem('passion-theme', next);
		const dark = next === 'dark' || (next === 'system' && matchMedia('(prefers-color-scheme: dark)').matches);
		document.documentElement.setAttribute('data-theme', dark ? 'dark' : 'light');
	}

	async function signOut() {
		try {
			await request('DELETE', '/api/v1/tokens/current');
		} catch {
			/* the token is dropped here either way */
		}
		clearToken();
		clearRunStorage();
		await goto('/login');
	}
</script>

{#snippet toggle(name: string, on: boolean, set: (on: boolean) => void)}
	<label class="flex min-h-[58px] cursor-pointer items-center justify-between gap-3 py-2 text-[15px] font-bold shadow-[inset_0_1px_0_var(--line)]">
		{name}
		<input type="checkbox" class="peer sr-only" checked={on} onchange={(e) => set(e.currentTarget.checked)} use:tickBox />
		<span
			class="relative h-8 w-[52px] shrink-0 rounded-full bg-well transition-colors peer-checked:bg-tint peer-focus-visible:outline-2 peer-focus-visible:outline-offset-2 peer-focus-visible:outline-ink before:absolute before:top-1 before:left-1 before:size-6 before:rounded-full before:bg-surface before:shadow-card-sm before:transition-transform peer-checked:before:translate-x-5"
			aria-hidden="true"
		></span>
	</label>
{/snippet}

<NavBar title="Profile" back={{ href: '/', label: 'Today' }} />

<div class="flex flex-col gap-3.5 px-4 pt-2 pb-8">
	<section class="rounded-3xl bg-surface px-[18px] py-2 shadow-card">
		<div class="flex flex-col gap-2.5 py-3">
			<h2 class="text-[15px] font-bold">Theme</h2>
			<Segmented label="Theme" items={themes.map((t) => ({ label: t.label, on: theme === t.value, onclick: () => setTheme(t.value) }))} />
		</div>

		{@render toggle('Timer sounds', sound, (on) => setSound((sound = on)))}
		{@render toggle('Haptics', haptics, (on) => setHaptics((haptics = on)))}
	</section>

	<Button variant="danger" onclick={signOut}>Log out</Button>
</div>
