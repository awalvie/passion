<script lang="ts" generics="T extends { value: string; label: string }">
	import type { Snippet } from 'svelte';
	import Icon from './Icon.svelte';

	// A choice from a list. A phone keeps its own picker; from lg the list opens
	// under the field in the app's own look. option draws a row of the list and
	// the chosen one in the field.
	let {
		value = $bindable(),
		options,
		label,
		option
	}: { value: string; options: T[]; label: string; option?: Snippet<[T]> } = $props();

	let open = $state(false);
	let active = $state(0);
	// The list is placed against the screen, so a sheet that scrolls cannot cut
	// it off. It opens upward when there is more room above.
	let place = $state('');
	let root: HTMLDivElement;
	let list = $state<HTMLUListElement>();
	const id = $props.id();

	const chosen = $derived(options.find((o) => o.value === value));

	function show() {
		active = Math.max(0, options.findIndex((o) => o.value === value));
		const r = root.getBoundingClientRect();
		const below = innerHeight - r.bottom - 24;
		const above = r.top - 24;
		const side = `left: ${r.left}px; width: ${r.width}px;`;
		place =
			below >= Math.min(320, above)
				? `${side} top: ${r.bottom + 8}px; max-height: ${Math.min(320, below)}px`
				: `${side} bottom: ${innerHeight - r.top + 8}px; max-height: ${Math.min(320, above)}px`;
		open = true;
	}

	function choose(o: T) {
		value = o.value;
		open = false;
		root.querySelector<HTMLButtonElement>('button')?.focus();
	}

	function key(e: KeyboardEvent) {
		if (!open) {
			if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
				e.preventDefault();
				show();
			}
			return;
		}
		if (e.key === 'Escape') {
			e.preventDefault();
			e.stopPropagation();
			open = false;
		} else if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
			e.preventDefault();
			active = (active + (e.key === 'ArrowDown' ? 1 : options.length - 1)) % options.length;
			list?.children[active]?.scrollIntoView({ block: 'nearest' });
		} else if (e.key === 'Enter' || e.key === ' ') {
			e.preventDefault();
			choose(options[active]);
		} else if (e.key === 'Tab') {
			open = false;
		}
	}

	// The list stays where it opened, so a scroll under it closes it.
	$effect(() => {
		if (!open) return;
		const close = (e: Event) => {
			if (!list?.contains(e.target as Node)) open = false;
		};
		addEventListener('scroll', close, true);
		addEventListener('resize', close);
		return () => {
			removeEventListener('scroll', close, true);
			removeEventListener('resize', close);
		};
	});

	function outside(e: PointerEvent) {
		if (open && !root.contains(e.target as Node)) open = false;
	}
</script>

<svelte:window onpointerdown={outside} />

<select class="input h-12 px-4 lg:hidden" bind:value aria-label={label}>
	{#each options as o (o.value)}
		<option value={o.value}>{o.label}</option>
	{/each}
</select>

<div class="relative hidden lg:block" bind:this={root}>
	<button
		type="button"
		class="input flex h-12 items-center justify-between gap-2.5 pr-3.5 pl-4 text-left"
		role="combobox"
		aria-label={label}
		aria-expanded={open}
		aria-controls="{id}-list"
		aria-activedescendant={open ? `${id}-${active}` : undefined}
		onclick={() => (open ? (open = false) : show())}
		onkeydown={key}
	>
		<span class="flex min-w-0 items-center gap-2.5 truncate">
			{#if chosen}
				{#if option}{@render option(chosen)}{:else}{chosen.label}{/if}
			{/if}
		</span>
		<span class="shrink-0 text-ink-2"><Icon name="chevron-down" size="1.125rem" /></span>
	</button>
	{#if open}
		<ul
			bind:this={list}
			id="{id}-list"
			role="listbox"
			aria-label={label}
			class="fixed z-50 overflow-y-auto rounded-[20px] bg-surface px-3.5 py-0.5 shadow-card"
			style={place}
		>
			{#each options as o, i (o.value)}
				<!-- The field takes the keys for the list, as a combobox does. -->
				<!-- svelte-ignore a11y_click_events_have_key_events -->
				<li
					id="{id}-{i}"
					role="option"
					aria-selected={o.value === value}
					class="list-row -mx-2 flex min-h-[52px] cursor-pointer items-center gap-3 rounded-xl px-2 text-[15px] font-bold {i === active ? 'bg-well' : ''}"
					onpointermove={() => (active = i)}
					onclick={() => choose(o)}
				>
					<span class="flex min-w-0 flex-1 items-center gap-3 truncate">
						{#if option}{@render option(o)}{:else}{o.label}{/if}
					</span>
					{#if o.value === value}<span class="shrink-0"><Icon name="check" size="1.125rem" stroke={3} /></span>{/if}
				</li>
			{/each}
		</ul>
	{/if}
</div>
