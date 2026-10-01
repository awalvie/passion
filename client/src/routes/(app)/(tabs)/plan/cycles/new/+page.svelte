<script lang="ts">
	import CycleForm from '$lib/CycleForm.svelte';
	import NavBar from '$lib/NavBar.svelte';

	let { data } = $props();
</script>

<svelte:head><title>New cycle</title></svelte:head>

<NavBar title="New cycle" back={{ href: '/plan', label: 'Plan' }} />

<CycleForm cycle={data.cycle} templates={data.templates} />
