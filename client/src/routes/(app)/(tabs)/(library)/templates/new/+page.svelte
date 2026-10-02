<script lang="ts">
	import { notice } from '$lib/notice.svelte';
	import { goto } from '$app/navigation';
	import { request } from '$lib/api';
	import TemplateEditor from '$lib/TemplateEditor.svelte';
	import type { SessionTemplate } from '$lib/template';

	let { data } = $props();
</script>

<svelte:head><title>New session template</title></svelte:head>

<TemplateEditor
	exercises={data.exercises}
	sources={data.sources}
	cancel="/templates"
	save={async (body) => {
		const created = await request<SessionTemplate>('POST', '/api/v1/session-templates', body);
		await goto(`/templates/${created.id}`);
		notice.text = 'Session saved';
	}}
/>
