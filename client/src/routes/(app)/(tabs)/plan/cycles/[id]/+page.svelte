<script lang="ts">
	import { goto } from '$app/navigation';
	import { describe, request } from '$lib/api';
	import Button from '$lib/Button.svelte';
	import CycleForm from '$lib/CycleForm.svelte';
	import FormError from '$lib/FormError.svelte';
	import NavBar from '$lib/NavBar.svelte';

	let { data } = $props();

	let busy = $state(false);
	let error = $state('');

	async function remove() {
		if (!confirm(`Delete ${data.cycle.name}? Its planned sessions go. The sessions you ran stay in History.`)) return;
		busy = true;
		error = '';
		try {
			await request('DELETE', `/api/v1/cycles/${data.cycle.id}`);
			await goto('/plan?view=cycles');
		} catch (e) {
			error = describe(e, (f) => f);
		} finally {
			busy = false;
		}
	}
</script>

<svelte:head><title>{data.cycle.name}</title></svelte:head>

<NavBar title={data.cycle.name} back={{ href: '/plan?view=cycles', label: 'Plan' }} />

{#key data.cycle.id}
	<CycleForm cycle={data.cycle} templates={data.templates}>
		<FormError message={error} />
		<Button variant="danger" disabled={busy} onclick={remove}>Delete cycle</Button>
	</CycleForm>
{/key}
