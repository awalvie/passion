<script lang="ts">
	import { plainText } from './text';

	let { text, class: className = '', outline = false }: { text: string; class?: string; outline?: boolean } = $props();

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

	// The outline reads the markdown itself: a paragraph that is only a bold
	// line or a # heading starts a section, and list items become a list.
	type Section = { title: string | null; blocks: { list: boolean; lines: string[] }[] };
	const heading = /^(?:#{1,6}\s+(.+)|(?:\*\*|__)(.+?)(?:\*\*|__):?)$/;
	const sections = $derived.by(() => {
		const out: Section[] = [{ title: null, blocks: [] }];
		for (const lines of paragraphs) {
			const h = lines.length === 1 ? lines[0].match(heading) : null;
			if (h) {
				out.push({ title: plainText(h[1] ?? h[2]), blocks: [] });
				continue;
			}
			const blocks = out.at(-1)!.blocks;
			for (const l of lines) {
				const list = listItem.test(l);
				const line = plainText(l.replace(listItem, '')).trim();
				if (list && blocks.at(-1)?.list) blocks.at(-1)!.lines.push(line);
				else blocks.push({ list, lines: [line] });
			}
		}
		return out.filter((s) => s.title || s.blocks.length);
	});
</script>

{#if outline}
	<div class={className}>
		{#each sections as section, i (i)}
			<section class="flex flex-col gap-2.5 py-4 first:pt-0 last:pb-0 [&+&]:border-t [&+&]:border-line">
				{#if section.title}
					<h3 class="font-[family-name:var(--font-digits)] text-[13px] font-bold tracking-[0.08em] text-ink-2 uppercase">{section.title}</h3>
				{/if}
				{#each section.blocks as block, j (j)}
					{#if block.list}
						<ul class="flex flex-col gap-2">
							{#each block.lines as line, k (k)}
								<li class="flex gap-3"><i class="mt-[0.55em] size-1.5 shrink-0 rounded-full bg-tint"></i><span class="min-w-0">{line}</span></li>
							{/each}
						</ul>
					{:else}
						<p>{block.lines.join(' ')}</p>
					{/if}
				{/each}
			</section>
		{/each}
	</div>
{:else}
	<div class="space-y-2 {className}">
		{#each paragraphs as lines, i (i)}
			<p>
				{#each lines as line, j (j)}{#if j}<br />{/if}{line}{/each}
			</p>
		{/each}
	</div>
{/if}
