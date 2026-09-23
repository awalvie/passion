<script lang="ts">
	import Notes from '$lib/Notes.svelte';
	import { choiceMeta, stepMeta, type Section } from '$lib/template';

	let { sections }: { sections: Section[] } = $props();
</script>

<section class="card card-pad">
	<h3 class="text-sm font-semibold">Plan</h3>
	<div class="mt-3 preview-flow">
		{#each sections as section, i (i)}
			<div class="preview-flow-item">
				<div class="preview-flow-marker" aria-hidden="true"></div>
				<div class="preview-flow-content">
					<div class="preview-flow-activity-title">
						<div class="text-sm font-semibold break-words">{section.name}</div>
						<div class="text-xs muted">
							{section.items.length} exercise{section.items.length === 1 ? '' : 's'}
						</div>
					</div>
					{#if section.notes}
						<Notes text={section.notes} class="mt-1 text-xs muted" />
					{/if}
					<div class="preview-flow-exercises">
						{#each section.items as item, j (j)}
							{#if item.step}
								<div class="preview-flow-exercise">
									<div class="preview-flow-exercise-name">{item.step.name}</div>
									<div class="preview-flow-exercise-meta text-xs muted">{stepMeta(item.step)}</div>
								</div>
							{:else}
								<div class="preview-flow-exercise">
									<div class="preview-flow-exercise-name">{item.choice.name}</div>
									<div class="preview-flow-exercise-meta text-xs muted">{choiceMeta(item.choice)}</div>
									{#if item.choice.notes}
										<Notes text={item.choice.notes} class="mt-1 text-xs muted" />
									{/if}
									<ul class="mt-1 space-y-1 border-l pl-2" style="border-color:var(--border)">
										{#each item.choice.options as option, k (k)}
											<li>
												<div class="text-xs font-medium break-words">{option.name}</div>
												<div class="text-[11px] muted">{stepMeta(option)}</div>
											</li>
										{/each}
									</ul>
								</div>
							{/if}
						{:else}
							<div class="text-xs muted">No exercises</div>
						{/each}
					</div>
				</div>
			</div>
		{:else}
			<div class="text-sm muted">No sections yet.</div>
		{/each}
	</div>
</section>
