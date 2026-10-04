<script lang="ts">
	import { notice } from '$lib/notice.svelte';
	import { goto } from '$app/navigation';
	import { request } from '$lib/api';
	import TemplateEditor from '$lib/TemplateEditor.svelte';

	let { data } = $props();
</script>

<svelte:head><title>Edit {data.template.name}</title></svelte:head>

<!-- The editor copies the template into its draft once, so another id needs a new editor. -->
{#key data.template.id}
	<TemplateEditor
		template={data.template}
		exercises={data.exercises}
		sources={data.sources}
		cancel="/templates/{data.template.id}"
		save={async (body) => {
			await request('PUT', `/api/v1/session-templates/${data.template.id}`, body);
			await goto(`/templates/${data.template.id}`);
			notice.text = 'Session saved';
		}}
	/>
{/key}
