<script lang="ts">
	// One filter of a library list. '' is the first option, which keeps every row.
	let {
		value = $bindable(),
		label,
		all,
		options,
		class: width
	}: { value: string; label: string; all: string; options: { value: string; label: string }[]; class: string } = $props();
</script>

<select bind:value aria-label={label} class="input min-h-11 min-w-0 rounded-full px-4 py-0 font-bold {width}">
	<option value="">{all}</option>
	{#each options as o (o.value)}
		<option value={o.value}>{o.label}</option>
	{/each}
</select>
