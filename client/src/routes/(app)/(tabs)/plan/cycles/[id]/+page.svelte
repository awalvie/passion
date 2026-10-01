<script lang="ts">
	import CycleForm from '$lib/CycleForm.svelte';
	import NavBar from '$lib/NavBar.svelte';

	let { data } = $props();
</script>

<svelte:head><title>{data.cycle.name}</title></svelte:head>

<NavBar title={data.cycle.name} back={{ href: '/plan', label: 'Plan' }} />

{#key data.cycle.id}
	<CycleForm cycle={data.cycle} templates={data.templates} />
{/key}
