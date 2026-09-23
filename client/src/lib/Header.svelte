<script lang="ts">
	import { afterNavigate, goto } from '$app/navigation';
	import { page } from '$app/state';
	import { request } from '$lib/api';
	import logo from '$lib/assets/logo.svg';
	import Icon from '$lib/Icon.svelte';
	import { clearToken } from '$lib/session';

	let menuOpen = $state(false);
	let dark = $state(document.documentElement.getAttribute('data-theme') === 'dark');

	// Every other page needs a signed-in account, so these are the only ones
	// without the nav.
	const authPage = $derived(['/login', '/signup'].includes(page.url.pathname));

	// V1 reloaded the page on every link. Here nothing does, so an open menu or
	// a focused dropdown has to be closed by hand.
	afterNavigate(() => {
		menuOpen = false;
		if (document.activeElement instanceof HTMLElement) document.activeElement.blur();
	});

	function setTheme() {
		const next = dark ? 'dark' : 'light';
		document.documentElement.setAttribute('data-theme', next);
		localStorage.setItem('passion-theme', next);
	}

	async function signOut() {
		try {
			await request('DELETE', '/api/v1/tokens/current');
		} catch {
			/* the token is dropped here either way */
		}
		clearToken();
		await goto('/login');
	}
</script>

{#snippet themeSwitch()}
	<label class="theme-switch" title="Toggle light/dark theme">
		<input type="checkbox" bind:checked={dark} onchange={setTheme} />
		<span class="theme-slider"></span>
	</label>
{/snippet}

<header class="site-header">
	<div class="site-header-top">
		<div class="min-w-0 flex-1">
			<a href="/" class="site-header-logo">
				<img src={logo} alt="" class="h-9 w-9 md:h-14 md:w-14" style="filter: var(--logo-filter)" />
				Passion
			</a>
		</div>

		<div class="flex shrink-0 items-center gap-2">
			{#if authPage}
				<div class="theme-toggle">
					<span class="muted hidden sm:inline">Light</span>
					{@render themeSwitch()}
					<span class="muted hidden sm:inline">Dark</span>
				</div>
			{:else}
				<label for="site-nav-toggle" class="site-header-menu-btn md:hidden" title="Menu">
					<span class="sr-only">Menu</span>
					<Icon name="menu" size="1.25rem" />
				</label>
			{/if}
		</div>
	</div>

	{#if !authPage}
		<input
			type="checkbox"
			id="site-nav-toggle"
			class="peer sr-only"
			bind:checked={menuOpen}
		/>

		<nav
			class="site-header-nav mt-2 hidden flex-col gap-0.5 border-t pt-3 peer-checked:flex md:mt-0 md:flex md:flex-row md:items-center md:justify-end md:gap-1 md:border-t-0 md:pt-2 md:peer-checked:flex"
			style="border-color: var(--border)"
			aria-label="Main navigation"
		>
			<div class="site-nav-dropdown">
				<button
					class="site-header-link site-nav-dropdown-toggle site-nav-dropdown-toggle-no-icon"
					aria-haspopup="true"
				>
					Library
				</button>
				<div
					class="site-nav-dropdown-menu site-nav-dropdown-menu-left site-nav-dropdown-menu-pills"
				>
					<p class="site-nav-group-label">Library</p>
					<a class="site-nav-dropdown-item" href="/templates">Sessions</a>
					<a class="site-nav-dropdown-item" href="/exercises">Exercises</a>
				</div>
			</div>
			<hr class="site-nav-divider md:hidden" />
			<div class="site-nav-dropdown">
				<button
					class="site-header-link site-nav-dropdown-toggle"
					aria-haspopup="true"
					aria-label="Account"
				>
					<Icon name="user" />
					<span class="md:hidden">Account</span>
				</button>
				<div class="site-nav-dropdown-menu site-nav-dropdown-menu-pills">
					<p class="site-nav-group-label">Account</p>
					<button type="button" class="site-nav-dropdown-item" onclick={signOut}>Log out</button>
					<div class="site-nav-theme-row">
						<span>Theme</span>
						{@render themeSwitch()}
					</div>
				</div>
			</div>
		</nav>
	{/if}
</header>
