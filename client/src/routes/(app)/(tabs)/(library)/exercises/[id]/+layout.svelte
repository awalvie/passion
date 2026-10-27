<script lang="ts">
	import { page } from '$app/state';

	let { children } = $props();
</script>

<!-- The page copies its exercise into form state once, so another id or a
     fresh load after a save needs a new page. -->
{#key page.data.exercise}
	{@render children()}
{/key}
