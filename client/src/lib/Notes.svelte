<script lang="ts">
	let { text, class: className = '' }: { text: string; class?: string } = $props();

	// Catalog files wrap their notes by hand. As in markdown, which V1 used, a
	// blank line starts a paragraph and a single newline is a space, except
	// before a list item.
	const listItem = /^(\d+[.)]|[-*•])\s/;
	const paragraphs = $derived(
		text.split(/\n\s*\n/).map((p) => {
			const lines: string[] = [];
			for (const raw of p.split('\n')) {
				const line = raw.trim();
				if (lines.length && !listItem.test(line)) lines[lines.length - 1] += ` ${line}`;
				else lines.push(line);
			}
			return lines;
		})
	);
</script>

<div class="space-y-2 {className}">
	{#each paragraphs as lines, i (i)}
		<p>
			{#each lines as line, j (j)}{#if j}<br />{/if}{line}{/each}
		</p>
	{/each}
</div>
