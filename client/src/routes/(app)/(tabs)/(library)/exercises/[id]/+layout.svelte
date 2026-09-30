<script lang="ts">
	import { page } from '$app/state';

	let { children } = $props();
</script>

<!-- The page copies its exercise into form state once, so another id needs a new page. -->
{#key page.params.id}
	{@render children()}
{/key}
