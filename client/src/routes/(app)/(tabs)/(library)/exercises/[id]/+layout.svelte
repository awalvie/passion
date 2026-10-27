<script lang="ts">
	import { page } from '$app/state';

	let { children } = $props();
</script>

<!-- The page copies its exercise into form state once, so another id or a
     save needs a new page. Any other reload keeps the page and its edits. -->
{#key `${page.params.id}:${page.state.saved ?? ''}`}
	{@render children()}
{/key}
