<script lang="ts">
	import { goto } from '$app/navigation';
	import { RequestFailed, request } from '$lib/api';
	import FormError from '$lib/FormError.svelte';
	import { setToken } from '$lib/session';

	type SignedUp = { token: { token: string } };

	const labels: Record<string, string> = {
		display_name: 'Name',
		email: 'Email',
		password: 'Password',
		timezone: 'Time zone'
	};

	let displayName = $state('');
	let email = $state('');
	let password = $state('');
	let confirm = $state('');
	let error = $state('');
	let busy = $state(false);

	async function signUp(event: SubmitEvent) {
		event.preventDefault();
		error = '';
		if (password !== confirm) {
			error = 'The two passwords do not match.';
			return;
		}
		busy = true;

		try {
			const created = await request<SignedUp>('POST', '/api/v1/accounts', {
				display_name: displayName,
				email,
				password,
				timezone: Intl.DateTimeFormat().resolvedOptions().timeZone
			});
			setToken(created.token.token);
			await goto('/');
		} catch (e) {
			error = describe(e);
		} finally {
			busy = false;
		}
	}

	function describe(e: unknown): string {
		if (!(e instanceof RequestFailed)) return 'Could not reach the server. Try again.';
		const fields = e.error.fields;
		if (fields) {
			return Object.entries(fields)
				.map(([key, problem]) => `${labels[key] ?? key} ${problem}.`)
				.join(' ');
		}
		// The server answers a taken address the same as any other failure, so
		// that nobody can ask it who has an account.
		if (e.status === 401) return 'That account could not be created. Check the details, or log in.';
		return e.error.message;
	}
</script>

<svelte:head><title>Sign up</title></svelte:head>

<section class="card card-pad max-w-md mx-auto">
	<h1 class="text-xl font-bold">Sign up</h1>
	<p class="text-sm muted mt-1">Create your account to save personal workouts.</p>

	<FormError message={error} />

	<form class="mt-4 space-y-3" onsubmit={signUp}>
		<div>
			<label for="display_name" class="text-xs muted">Name</label>
			<input
				id="display_name"
				name="display_name"
				autocomplete="name"
				class="mt-1 w-full input"
				bind:value={displayName}
				required
			/>
		</div>
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
				autocomplete="new-password"
				class="mt-1 w-full input"
				minlength="8"
				maxlength="128"
				bind:value={password}
				required
			/>
		</div>
		<div>
			<label for="password_confirm" class="text-xs muted">Confirm password</label>
			<input
				id="password_confirm"
				name="password_confirm"
				type="password"
				autocomplete="new-password"
				class="mt-1 w-full input"
				minlength="8"
				maxlength="128"
				bind:value={confirm}
				required
			/>
		</div>
		<button type="submit" class="btn btn-accent" disabled={busy}>
			{busy ? 'Creating account…' : 'Create account'}
		</button>
	</form>

	<p class="text-sm muted mt-4">Already registered? <a href="/login" class="underline">Log in</a>.</p>
</section>
