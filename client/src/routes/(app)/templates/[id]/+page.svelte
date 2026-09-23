<script lang="ts">
	import Icon from '$lib/Icon.svelte';
	import Notes from '$lib/Notes.svelte';
	import { choiceMeta, stepMeta } from '$lib/template';

	let { data } = $props();

	const t = $derived(data.template);
	const locked = $derived(
		t.shipped
			? 'The app ships this session, so it cannot be changed.'
			: t.retired_at
				? 'You retired this session.'
				: ''
	);
</script>

<svelte:head><title>{t.name}</title></svelte:head>

<div class="space-y-4">
	<div class="card card-pad">
		<div class="flex items-start gap-2">
			{#if t.color}
				<span class="inline-block w-3 h-3 rounded-full shrink-0 mt-2" style="background:{t.color}"></span>
			{/if}
			<div class="min-w-0 flex-1">
				<h1 class="text-xl font-bold m-0 break-words">{t.name}</h1>
				{#if t.tags.length}
					<div class="mt-1 text-[11px] muted">{t.tags.join(' · ')}</div>
				{/if}
			</div>
			<a
				class="rounded-md btn-ghost p-2 inline-flex items-center justify-center shrink-0"
				href="/templates"
				title="Back to templates"
				aria-label="Back to templates"
			>
				<Icon name="arrow-left" />
			</a>
		</div>

		{#if locked}
			<p class="mt-2 text-xs muted">{locked}</p>
		{/if}

		{#if t.source || t.needs}
			<div class="mt-3 flex flex-wrap items-center gap-2 text-[11px] muted">
				{#if t.source}
					<span
						class="inline-flex items-center gap-1 font-medium px-1.5 py-0.5 rounded"
						style="background:var(--accent-bg);color:var(--accent)"
					>
						<Icon name="book-marked" size="0.6rem" />{t.source}
					</span>
				{/if}
				{#if t.needs}
					<span>Needs: {t.needs}</span>
				{/if}
			</div>
		{/if}

		{#if t.notes}
			<Notes text={t.notes} class="mt-3 text-sm" />
		{/if}
	</div>

	<section class="card card-pad">
		<h3 class="text-sm font-semibold">Plan</h3>
		<div class="mt-3 preview-flow">
			{#each t.sections as section, i (i)}
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
</div>
