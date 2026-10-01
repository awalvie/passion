<script lang="ts">
	let { message }: { message: string } = $props();
</script>

{#if message}
	<div class="mt-3 rounded-2xl bg-bad/10 px-4 py-3 text-[15px] font-semibold text-bad" role="alert">
		{message}
	</div>
{/if}
