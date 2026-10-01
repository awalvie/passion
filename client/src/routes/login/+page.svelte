<script lang="ts">
	import { goto } from '$app/navigation';
	import { RequestFailed, request } from '$lib/api';
	import Button from '$lib/Button.svelte';
	import FormError from '$lib/FormError.svelte';
	import Topo from '$lib/Topo.svelte';
	import { topTopo } from '$lib/topo';
	import { setToken } from '$lib/session';

	type Issued = { token: string; expires_at: string };

	let email = $state('');
	let password = $state('');
	let error = $state('');
	let busy = $state(false);

	async function signIn(event: SubmitEvent) {
		event.preventDefault();
		error = '';
		busy = true;

		try {
			const issued = await request<Issued>('POST', '/api/v1/tokens', { email, password });
			setToken(issued.token);
			await goto('/');
		} catch (e) {
			error =
				e instanceof RequestFailed ? e.error.message : 'Could not reach the server. Try again.';
		} finally {
			busy = false;
		}
	}
</script>

<svelte:head><title>Log in</title></svelte:head>

<Topo
	shape={topTopo}
	class="absolute inset-x-0 top-0 h-80 w-full text-[var(--topo)] [mask-image:linear-gradient(#000_30%,transparent)]"
/>
<section class="relative mx-auto flex max-w-[430px] flex-col px-4 pt-[calc(env(safe-area-inset-top)+4rem)] pb-8">
	<h1 class="text-[32px] leading-[1.1] font-extrabold tracking-[-0.02em]">Log in</h1>
	<p class="mt-1 text-[15px] font-semibold text-ink-2">Sign in to access your workouts.</p>

	<FormError message={error} />

	<form class="mt-6 flex flex-col gap-4 rounded-3xl bg-surface p-5 shadow-card" onsubmit={signIn}>
		<div>
			<label for="email" class="text-xs font-bold text-ink-2">Email</label>
			<input
				enterkeyhint="go"
				id="email"
				name="email"
				type="email"
				autocomplete="username"
				class="mt-1 w-full input"
				bind:value={email}
				required
			/>
		</div>
		<div>
			<label for="password" class="text-xs font-bold text-ink-2">Password</label>
			<input
				enterkeyhint="go"
				id="password"
				name="password"
				type="password"
				autocomplete="current-password"
				class="mt-1 w-full input"
				bind:value={password}
				required
			/>
		</div>
		<Button type="submit" disabled={busy}>{busy ? 'Signing in…' : 'Log in'}</Button>
	</form>

	<p class="mt-5 text-center text-[15px] font-semibold text-ink-2">
		No account yet? <a href="/signup" class="font-bold text-link underline">Sign up</a>
	</p>
</section>
