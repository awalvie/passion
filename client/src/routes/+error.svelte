<script lang="ts">
	import { page } from '$app/state';
</script>

<svelte:head><title>{page.status === 404 ? 'Not found' : 'Something went wrong'}</title></svelte:head>

<section class="card card-pad max-w-md mx-auto text-center">
	<h1 class="text-xl font-bold">{page.status === 404 ? 'Not found' : 'Something went wrong'}</h1>
	<p class="text-sm muted mt-1">
		{page.status === 404 ? 'That page does not exist, or it is not yours.' : page.error?.message}
	</p>
	<a href="/" class="btn btn-ghost mt-4">Home</a>
</section>
