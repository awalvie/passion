<script lang="ts">
	import { goto } from '$app/navigation';
	import { request } from '$lib/api';
	import { setSound, soundOn } from '$lib/audio';
	import Button from '$lib/Button.svelte';
	import NavBar from '$lib/NavBar.svelte';
	import { clearRunStorage, clearToken } from '$lib/session';

	type Theme = 'system' | 'light' | 'dark';
	const themes: { value: Theme; label: string }[] = [
		{ value: 'system', label: 'System' },
		{ value: 'light', label: 'Light' },
		{ value: 'dark', label: 'Dark' }
	];

	let sound = $state(soundOn());

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

<NavBar title="Settings" back={{ href: '/', label: 'Today' }} />

<div class="flex flex-col gap-6 px-4 pt-4 pb-8">
	<section class="flex flex-col gap-2">
		<h2 class="px-1 text-sm text-ink-2">Theme</h2>
		<div class="grid grid-cols-3 gap-1 rounded-xl bg-surface p-1 shadow-sm">
			{#each themes as t (t.value)}
				<button
					type="button"
					class="h-10 rounded-lg text-base {theme === t.value ? 'bg-tint font-semibold text-on-tint' : 'text-ink'}"
					aria-pressed={theme === t.value}
					onclick={() => setTheme(t.value)}
				>
					{t.label}
				</button>
			{/each}
		</div>
	</section>

	<label class="flex items-center justify-between rounded-xl bg-surface px-4 py-3 text-base shadow-sm">
		Timer sounds
		<input
			type="checkbox"
			class="size-6 accent-tint"
			checked={sound}
			onchange={(e) => setSound((sound = e.currentTarget.checked))}
		/>
	</label>

	<Button variant="danger" onclick={signOut}>Log out</Button>
</div>
