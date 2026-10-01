<script lang="ts">
	import { goto } from '$app/navigation';
	import { describe, request } from '$lib/api';
	import { cycleWeek } from '$lib/dates';
	import FormError from '$lib/FormError.svelte';
	import Icon, { type IconName } from '$lib/Icon.svelte';
	import Menu from '$lib/Menu.svelte';
	import NavBar from '$lib/NavBar.svelte';

	let { data } = $props();

	let error = $state('');

	const span = (a: string, b: string) => Math.round((Date.parse(b) - Date.parse(a)) / 86_400_000);
	const utc = (date: string, o: Intl.DateTimeFormatOptions) =>
		new Date(`${date}T00:00:00Z`).toLocaleDateString(undefined, { ...o, timeZone: 'UTC' });
	const short = (date: string) => utc(date, { day: 'numeric', month: 'short' });

	const when = $derived.by(() => {
		const c = data.cycle;
		if (data.today > c.ends) return 'Ended';
		if (data.today < c.starts) {
			const n = span(data.today, c.starts);
			return n === 1 ? 'Starts tomorrow' : `Starts in ${n} days`;
		}
		const w = cycleWeek(c.starts, c.ends, data.today);
		return `Week ${w.week} of ${w.of}`;
	});

	const rows: { icon: IconName; label: string; value: string }[] = $derived.by(() => {
		const c = data.cycle;
		const repeats = Math.ceil((span(c.block_from, c.ends) + 1) / c.block_days);
		const weeks = Math.round((span(c.starts, c.ends) + 1) / 7);
		return [
			{ icon: 'pencil', label: 'Name', value: c.name },
			{ icon: 'calendar', label: 'Dates', value: `${short(c.starts)} – ${short(c.ends)} · ${weeks === 1 ? '1 week' : `${weeks} weeks`}` },
			{ icon: 'layers2', label: 'Block', value: `${c.block_days === 1 ? '1 day' : `${c.block_days} days`}, repeats ${repeats === 1 ? 'once' : `${repeats} times`}` }
		];
	});

	async function remove() {
		if (!confirm(`Delete ${data.cycle.name}? Its planned sessions go. The sessions you ran stay in History.`)) return;
		error = '';
		try {
			await request('DELETE', `/api/v1/cycles/${data.cycle.id}`);
			await goto('/plan?view=cycles');
		} catch (e) {
			error = describe(e, (f) => f);
		}
	}
</script>

<svelte:head><title>{data.cycle.name}</title></svelte:head>

<NavBar back={{ href: '/plan?view=cycles', label: 'Plan' }}>
	{#snippet actions()}
		<Menu items={[{ label: 'Delete cycle', danger: true, onclick: remove }]} />
	{/snippet}
</NavBar>

<div class="flex flex-col gap-3.5 px-4 pt-1 pb-[calc(env(safe-area-inset-bottom)+5rem)]">
	<header class="px-1">
		<h1 class="text-[32px] leading-[1.1] font-extrabold tracking-[-0.02em] break-words">{data.cycle.name}</h1>
		<p class="mt-1 text-[15px] font-semibold text-ink-2">
			{short(data.cycle.starts)} – {short(data.cycle.ends)} · {when}
		</p>
	</header>

	<FormError message={error} />

	<section class="flex flex-col gap-2">
		<h2 class="px-1 text-[15px] font-bold">Cycle</h2>
		<ul class="rounded-3xl bg-surface px-[18px] shadow-card">
			{#each rows as r (r.label)}
				<li class="flex min-h-[58px] items-center gap-3.5 py-2 [&:not(:first-child)]:shadow-[inset_0_1px_0_var(--line)]">
					<span class="flex size-9 shrink-0 items-center justify-center rounded-xl bg-well text-ink-2">
						<Icon name={r.icon} size="1.125rem" />
					</span>
					<span class="min-w-0 flex-1">
						<span class="block text-[15px] font-bold">{r.label}</span>
						<span class="mt-0.5 block truncate text-xs font-semibold text-ink-2">{r.value}</span>
					</span>
				</li>
			{/each}
		</ul>
	</section>
</div>
