<script lang="ts">
	import NavBar from '$lib/NavBar.svelte';
	import PastGoals from '$lib/PastGoals.svelte';

	let { data } = $props();
	const done = $derived(data.period.goals.filter((g) => g.done).length);
</script>

<svelte:head><title>{data.period.title}</title></svelte:head>

<NavBar back={{ href: '/plan?view=goals', label: 'Goals' }} />

<div class="flex flex-col gap-3.5 px-4 pt-2 pb-[calc(env(safe-area-inset-bottom)+5rem)]">
	<header class="px-1">
		<h1 class="page-title break-words">{data.period.title}</h1>
		<p class="mt-1 text-[15px] font-semibold text-ink-2">{data.period.note} · {done} of {data.period.goals.length} done</p>
	</header>
	<PastGoals period={data.period} />
</div>
