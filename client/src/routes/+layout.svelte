<script lang="ts">
	import favicon from '$lib/assets/favicon.svg';
	import Header from '$lib/Header.svelte';
	import '../app.css';

	let { children } = $props();
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
</svelte:head>

<div class="passion-container">
	<Header />

	<main class="mt-4 md:mt-6">
		{@render children()}
	</main>

	<footer class="mt-10 divider pt-4 text-xs muted">
		<span>Passion — plan, run, and log your climbing training.</span>
	</footer>
</div>
