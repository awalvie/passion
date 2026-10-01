<script lang="ts">
	import { page } from '$app/state';
	import Button from '$lib/Button.svelte';
</script>

<svelte:head><title>{page.status === 404 ? 'Not found' : 'Something went wrong'}</title></svelte:head>

<section class="mx-auto flex max-w-[430px] flex-col gap-2 px-4 pt-[calc(env(safe-area-inset-top)+4rem)]">
	<h1 class="text-[32px] leading-[1.1] font-extrabold tracking-[-0.02em]">
		{page.status === 404 ? 'Not found' : 'Something went wrong'}
	</h1>
	<p class="text-[15px] font-semibold text-ink-2">
		{page.status === 404 ? 'That page does not exist, or it is not yours.' : page.error?.message}
	</p>
	<div class="mt-4"><Button variant="secondary" href="/">Home</Button></div>
</section>
