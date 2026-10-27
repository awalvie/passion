<script lang="ts">
	import NavBar from '$lib/NavBar.svelte';
	import RunView from '$lib/RunView.svelte';

	let { data } = $props();

	const run = $derived(data.run);
</script>

<svelte:head><title>{run.name}</title></svelte:head>

<NavBar back={{ href: '/history', label: 'History' }}>
	{#snippet actions()}
		<a href="/run/{run.id}" class="flex h-11 items-center rounded-full bg-surface px-4 text-[15px] font-bold text-ink shadow-card-sm">Edit</a>
	{/snippet}
</NavBar>

<div class="px-4 pt-2">
	<RunView {run} exerciseHref={(s) => `/history/exercise/${s.exercise}?from=/history/${run.id}&name=${encodeURIComponent(s.name)}`} />
</div>
