<script lang="ts">
	import { untrack } from 'svelte';
	import Segmented from '$lib/Segmented.svelte';
	import { goto } from '$app/navigation';
	import { describe, request } from '$lib/api';
	import { setSound, soundOn } from '$lib/audio';
	import Button from '$lib/Button.svelte';
	import FormError from '$lib/FormError.svelte';
	import { forgetGrades } from '$lib/grades';
	import NavBar from '$lib/NavBar.svelte';
	import { hapticsOn, setHaptics, tickBox } from '$lib/haptics';
	import { clearRunStorage, clearToken } from '$lib/session';

	let { data } = $props();

	const scaleNames: Record<string, string> = { font: 'Font', v: 'V scale', french: 'French', yds: 'YDS' };
	let boulder = $state(untrack(() => data.account.boulder_grades));
	let route = $state(untrack(() => data.account.route_grades));
	let gradeError = $state('');

	async function setScale(kind: 'boulder' | 'route', system: string) {
		const before = { boulder, route };
		if (kind === 'boulder') boulder = system;
		else route = system;
		gradeError = '';
		try {
			await request('PUT', '/api/v1/accounts/me/grades', { boulder_grades: boulder, route_grades: route });
			forgetGrades();
		} catch (e) {
			({ boulder, route } = before);
			gradeError = describe(e, (f) => (f === 'boulder_grades' ? 'The boulder scale' : 'The route scale'));
		}
	}

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

<svelte:head><title>Profile</title></svelte:head>

<NavBar back={{ href: '/', label: 'Today' }} />

<div class="flex flex-col gap-3.5 px-4 pt-2 pb-8">
	<header class="px-1">
		<h1 class="text-[32px] leading-[1.1] font-extrabold tracking-[-0.02em] break-words">{data.account.display_name || data.account.email}</h1>
		{#if data.account.display_name}
			<p class="mt-1 truncate text-[15px] font-semibold text-ink-2">{data.account.email}</p>
		{/if}
	</header>

	<section class="flex flex-col gap-2">
		<h2 class="px-1 text-[15px] font-bold">Grades</h2>
		<div class="flex flex-col gap-3.5 rounded-3xl bg-surface p-[18px] shadow-card">
			{#each [{ kind: 'boulder', label: 'Boulders', on: boulder }, { kind: 'route', label: 'Routes', on: route }] as const as g (g.kind)}
				<div class="flex flex-col gap-2">
					<span class="text-xs font-semibold text-ink-2">{g.label}</span>
					<Segmented
						label="{g.label} scale"
						items={data.grades.scales
							.filter((s) => s.boulder === (g.kind === 'boulder'))
							.map((s) => ({
								label: `${scaleNames[s.system] ?? s.system} · ${s.grades[Math.floor(s.grades.length / 2)]}`,
								on: g.on === s.system,
								onclick: () => setScale(g.kind, s.system)
							}))}
					/>
				</div>
			{/each}
			<FormError message={gradeError} />
		</div>
	</section>

	<section class="flex flex-col gap-2">
		<h2 class="px-1 text-[15px] font-bold">App</h2>
		<div class="rounded-3xl bg-surface px-[18px] py-2 shadow-card">
			<div class="flex flex-col gap-2.5 py-3">
				<h3 class="text-[15px] font-bold">Theme</h3>
				<Segmented label="Theme" items={themes.map((t) => ({ label: t.label, on: theme === t.value, onclick: () => setTheme(t.value) }))} />
			</div>

			{@render toggle('Timer sounds', sound, (on) => setSound((sound = on)))}
			{@render toggle('Haptics', haptics, (on) => setHaptics((haptics = on)))}
		</div>
	</section>

	<Button variant="danger" onclick={signOut}>Log out</Button>
</div>
