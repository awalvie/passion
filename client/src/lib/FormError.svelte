<script lang="ts">
	let { message }: { message: string } = $props();
</script>

{#if message}
	<div
		class="mt-3 rounded-md border px-3 py-2 text-sm"
		style="border-color: var(--destructive); color: var(--destructive)"
		role="alert"
	>
		{message}
	</div>
{/if}
