<script lang="ts">
	import { goto } from '$app/navigation';
	import { RequestFailed, describe, request } from '$lib/api';
	import Button from '$lib/Button.svelte';
	import FormError from '$lib/FormError.svelte';
	import Topo from '$lib/Topo.svelte';
	import { topTopo } from '$lib/topo';
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
			// The server answers a taken address the same as any other failure, so
			// that nobody can ask it who has an account.
			error =
				e instanceof RequestFailed && e.status === 401
					? 'That account could not be created. Check the details, or log in.'
					: describe(e, (field) => labels[field] ?? field);
		} finally {
			busy = false;
		}
	}
</script>

<svelte:head><title>Sign up</title></svelte:head>

<Topo
	shape={topTopo}
	class="absolute inset-x-0 top-0 h-80 w-full text-[var(--topo)] [mask-image:linear-gradient(#000_30%,transparent)]"
/>
<section class="relative mx-auto flex max-w-[430px] flex-col px-4 pt-[calc(env(safe-area-inset-top)+4rem)] pb-8">
	<h1 class="text-[32px] leading-[1.1] font-extrabold tracking-[-0.02em]">Sign up</h1>
	<p class="mt-1 text-[15px] font-semibold text-ink-2">Create your account to save personal workouts.</p>

	<FormError message={error} />

	<form class="mt-6 flex flex-col gap-4 rounded-3xl bg-surface p-5 shadow-card" onsubmit={signUp}>
		<div>
			<label for="display_name" class="text-xs font-bold text-ink-2">Name</label>
			<input
				enterkeyhint="go"
				id="display_name"
				name="display_name"
				autocomplete="name"
				class="mt-1 w-full input"
				bind:value={displayName}
				required
			/>
		</div>
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
				autocomplete="new-password"
				class="mt-1 w-full input"
				minlength="8"
				maxlength="128"
				bind:value={password}
				required
			/>
		</div>
		<div>
			<label for="password_confirm" class="text-xs font-bold text-ink-2">Confirm password</label>
			<input
				enterkeyhint="go"
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
		<Button type="submit" disabled={busy}>{busy ? 'Creating account…' : 'Create account'}</Button>
	</form>

	<p class="mt-5 text-center text-[15px] font-semibold text-ink-2">
		Already registered? <a href="/login" class="font-bold text-link underline">Log in</a>
	</p>
</section>
