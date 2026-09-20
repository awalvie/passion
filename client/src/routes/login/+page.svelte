<script lang="ts">
	import { goto } from '$app/navigation';
	import { RequestFailed, request } from '$lib/api';
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

<main class="p-6">
	<section class="card card-pad max-w-md mx-auto">
		<h1 class="text-xl font-bold">Log in</h1>
		<p class="text-sm muted mt-1">Sign in to access your workouts.</p>

		{#if error}
			<div
				class="mt-3 rounded-md border px-3 py-2 text-sm"
				style="border-color: var(--destructive); color: var(--destructive)"
				role="alert"
			>
				{error}
			</div>
		{/if}

		<form class="mt-4 space-y-3" onsubmit={signIn}>
			<div>
				<label for="email" class="text-xs muted">Email</label>
				<input
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
				<label for="password" class="text-xs muted">Password</label>
				<input
					id="password"
					name="password"
					type="password"
					autocomplete="current-password"
					class="mt-1 w-full input"
					bind:value={password}
					required
				/>
			</div>
			<button type="submit" class="btn btn-accent" disabled={busy}>
				{busy ? 'Signing in…' : 'Log in'}
			</button>
		</form>

		<p class="text-sm muted mt-4">No account yet? <a href="/signup" class="underline">Sign up</a>.</p>
	</section>
</main>
