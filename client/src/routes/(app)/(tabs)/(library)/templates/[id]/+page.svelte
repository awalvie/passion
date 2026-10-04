<script lang="ts">
	import { goto } from '$app/navigation';
	import { describe, request } from '$lib/api';
	import Button from '$lib/Button.svelte';
	import { saveCentre, type Centre } from '$lib/centres';
	import { haptic } from '$lib/haptics';
	import FormError from '$lib/FormError.svelte';
	import Icon from '$lib/Icon.svelte';
	import Menu, { type MenuItem } from '$lib/Menu.svelte';
	import { page } from '$app/state';
	import Notes from '$lib/Notes.svelte';
	import { stepsOf } from '$lib/run';
	import { startRun } from '$lib/runState.svelte';
	import Segmented from '$lib/Segmented.svelte';
	import { sessionIcons } from '$lib/SessionIcon.svelte';
	import { choiceMeta, stepMeta, templateFieldLabel, type Choice, type SessionTemplate } from '$lib/template';
	import { formatDate, weekday } from '$lib/dates';
	import { plainText } from '$lib/text';
	import Topo from '$lib/Topo.svelte';
	import { heroTopo } from '$lib/topo';

	let { data } = $props();

	const t = $derived(data.template);
	const locked = $derived(
		t.shipped
			? 'The app ships this template, so it cannot be changed. Duplicate it to make your own.'
			: t.retired_at
				? 'You retired this template.'
				: ''
	);

	let error = $state('');
	let doing = $state<'duplicate' | 'start' | 'retire' | null>(null);

	// A copy is a new template of your own. Its steps keep the exercise ids, so
	// both copies still count toward the same exercises.
	async function duplicate() {
		if (doing) return;
		error = '';
		doing = 'duplicate';
		try {
			const { id, shipped, retired_at, ...fields } = t;
			const copy = await request<SessionTemplate>('POST', '/api/v1/session-templates', {
				...fields,
				name: `${t.name} (copy)`
			});
			await goto(`/templates/${copy.id}`);
		} catch (e) {
			error = describe(e, templateFieldLabel);
		} finally {
			doing = null;
		}
	}

	async function start() {
		error = '';
		doing = 'start';
		try {
			const run = await startRun({ template: t.id });
			await goto(`/run/${run.id}`);
		} catch (e) {
			error = describe(e, templateFieldLabel);
		} finally {
			doing = null;
		}
	}

	async function retire() {
		if (doing) return;
		if (!confirm(`Retire “${t.name}”? It leaves your list of session templates.`)) return;
		error = '';
		doing = 'retire';
		try {
			await request('POST', `/api/v1/session-templates/${t.id}/retire`);
			await goto('/templates');
		} catch (e) {
			error = describe(e, templateFieldLabel);
		} finally {
			doing = null;
		}
	}

	// A tap on a centre says whether this session can be done there.
	let centres = $state<Centre[]>([]);
	$effect.pre(() => {
		centres = structuredClone($state.snapshot(data.centres));
	});

	async function toggleCentre(i: number) {
		const before = $state.snapshot(centres[i]);
		const on = before.sessions.includes(t.id);
		centres[i].sessions = on ? before.sessions.filter((s) => s !== t.id) : [...before.sessions, t.id];
		error = '';
		try {
			await saveCentre($state.snapshot(centres[i]));
		} catch (e) {
			centres[i] = before;
			error = describe(e, () => 'That centre');
		}
	}

	const menu = $derived<MenuItem[]>([
		...(locked ? [] : [{ label: 'Edit', onclick: () => goto(`/templates/${t.id}/edit`) }]),
		{ label: 'Duplicate', onclick: duplicate },
		...(locked ? [] : [{ label: 'Retire', danger: true, onclick: retire }])
	]);

	const tab = $derived(page.url.searchParams.get('tab') === 'exercises' ? 'exercises' : 'overview');
	const icon = $derived(sessionIcons.find(([name]) => name === t.icon)?.[0]);
	// A pick-one section carries its badge in its own header, not a second title.
	const only = (section: SessionTemplate['sections'][number]) =>
		section.items.length === 1 ? section.items[0].choice : undefined;
	const pickLabel = (c: Choice) =>
		c.pick > 0 && c.pick < c.options.length ? `Pick ${c.pick} of ${c.options.length}` : choiceMeta(c);
	const minutes = $derived.by(() => {
		const timed = data.runs.flatMap((r) => (r.elapsed_seconds ? [r.elapsed_seconds] : []));
		return timed.length ? Math.round(timed.reduce((a, b) => a + b, 0) / timed.length / 60) : null;
	});
	const day = (date: string) => `${weekday(date)} ${Number(date.slice(8))} ${formatDate(date, { month: 'short' })}`;

	// Long lists show three and fold open in place.
	let unfolded = $state<string[]>([]);
	const shown = <T,>(key: string, list: T[]) => (unfolded.includes(key) ? list : list.slice(0, 3));

	// The newest run's own note, or else the last note on one of its steps.
	const note = $derived.by(() => {
		if (!data.last) return '';
		const steps = stepsOf(data.last);
		return plainText(data.last.notes || steps.findLast((s) => s.run_notes)?.run_notes || '');
	});

	// Each weighted exercise of the newest run, at its heaviest set.
	const loads = $derived.by(() => {
		if (!data.last) return [];
		const names = new Map(stepsOf(data.last).map((s) => [s.exercise, s.name]));
		return [...new Set(data.last.sets.map((x) => x.exercise))].flatMap((exercise) => {
			const sets = data.last!.sets.filter((x) => x.exercise === exercise);
			const top = sets.reduce((a, b) => ((b.weight_kg ?? -Infinity) > (a.weight_kg ?? -Infinity) ? b : a));
			if (top.weight_kg === null) return [];
			const dose = top.reps ? `${sets.length}×${top.reps}` : `${sets.length} set${sets.length === 1 ? '' : 's'}`;
			return [{ exercise, name: names.get(exercise) ?? 'Exercise', text: `${dose} · ${top.weight_kg} kg` }];
		});
	});

	// The option after the one the newest run picked, from the first pick-one choice.
	const drill = $derived.by(() => {
		const choice = t.sections
			.flatMap((sec) => sec.items.flatMap((i) => (i.choice ? [i.choice] : [])))
			.find((c) => c.pick > 0 && c.pick < c.options.length);
		if (!choice || !data.last) return null;
		const picked = stepsOf(data.last).find((s) => s.from_choice && choice.options.some((o) => o.exercise === s.exercise));
		if (!picked) return null;
		const at = choice.options.findIndex((o) => o.exercise === picked.exercise);
		return { next: choice.options[(at + 1) % choice.options.length], last: picked };
	});

	const planned = $derived.by(() => {
		const d = data.next?.day;
		if (!d) return '';
		return d.local_date === data.next!.today ? 'Planned today' : `Planned ${weekday(d.local_date)} ${Number(d.local_date.slice(8))}`;
	});

	const steps = (section: SessionTemplate['sections'][number]) =>
		section.items.reduce((n, item) => n + (item.step ? 1 : item.choice.options.length), 0);
	// The first two exercise names of a section, then how many more.
	const firstNames = (section: SessionTemplate['sections'][number]) => {
		const names = section.items.flatMap((item) => (item.step ? [item.step.name] : item.choice.options.map((o) => o.name)));
		return names.slice(0, 2).join(', ') + (names.length > 2 ? ` +${names.length - 2}` : '');
	};
	// A section opened from the Overview's contents.
	const opened = $derived(Number(page.url.searchParams.get('open') ?? -1));
</script>

<svelte:head><title>{t.name}</title></svelte:head>

<div class="flex min-h-dvh flex-col">
	<header class="relative bg-[radial-gradient(90%_70%_at_88%_0%,var(--hero-2),var(--hero)_70%)] pt-[env(safe-area-inset-top)] pb-12 text-on-hero">
		<div class="absolute inset-0 overflow-hidden" aria-hidden="true">
			<Topo shape={heroTopo} class="h-full w-full text-[var(--hero-topo)]" />
		</div>
		<div class="relative flex h-14 items-center justify-between px-4">
			<a href="/templates" data-back class="inline-flex h-11 items-center gap-0.5 rounded-full bg-white/10 pr-4 pl-2.5 text-[15px] font-bold">
				<Icon name="chevron-left" size="1.25rem" stroke={2.4} />
				Sessions
			</a>
			<Menu items={menu} look="bg-white/10 text-on-hero" />
		</div>
		<div class="relative px-5 pt-3">
			{#if icon}
				<span
					class="mb-4 flex size-14 items-center justify-center rounded-[18px] {t.color ? '' : 'bg-white/10 text-tint'}"
					style={t.color ? `background: color-mix(in srgb, ${t.color} 30%, transparent); color: color-mix(in srgb, ${t.color} 45%, white)` : undefined}
				>
					<Icon name={icon} size="1.75rem" />
				</span>
			{/if}
			<h1 class="text-[40px] leading-[1.05] font-extrabold tracking-[-0.02em] break-words">{t.name}</h1>
			{#if t.tags.length || t.source || t.needs || t.shipped || minutes || planned}
				<div class="mt-3 flex flex-wrap gap-2">
					{#if planned}
						<span class="flex h-8 items-center rounded-full bg-tint px-3 text-[15px] font-bold text-on-tint">{planned}</span>
					{/if}
					{#if minutes}
						<span class="flex h-8 items-center rounded-full bg-white/10 px-3 text-[15px] font-bold">~{minutes} min</span>
					{/if}
					{#each t.tags as tag (tag)}
						<span class="flex h-8 items-center rounded-full bg-white/10 px-3 text-[15px] font-semibold">{tag}</span>
					{/each}
					{#if t.source}
						<span class="flex h-8 items-center gap-1.5 rounded-full bg-white/10 px-3 text-[15px] font-semibold">
							<Icon name="book-marked" size="0.875rem" />{t.source}
						</span>
					{/if}
					{#if t.needs}
						<span class="flex h-8 items-center gap-1.5 rounded-full px-3 text-[15px] font-semibold shadow-[inset_0_0_0_1.5px_rgba(242,246,234,0.2)]">
							<Icon name="backpack" size="0.875rem" />{t.needs}
						</span>
					{/if}
					{#if t.shipped}
						<span class="flex h-8 items-center gap-1.5 rounded-full px-3 text-[15px] font-semibold shadow-[inset_0_0_0_1.5px_rgba(242,246,234,0.2)]">
							<Icon name="lock" size="0.875rem" />Built in
						</span>
					{/if}
				</div>
			{/if}
		</div>
	</header>

	<div class="relative -mt-7 flex flex-1 flex-col gap-3.5 rounded-t-[28px] bg-ground px-4 pt-4">
		<Segmented
			label="Session"
			replace
			items={[
				{ label: 'Overview', href: `/templates/${t.id}`, on: tab === 'overview' },
				{ label: 'Exercises', href: `/templates/${t.id}?tab=exercises`, on: tab === 'exercises' }
			]}
		/>
		<FormError message={error} />

		{#if tab === 'overview'}
			{#if locked && !t.shipped}
				<p class="px-1 text-xs font-semibold text-ink-2">{locked}</p>
			{/if}
			{#if t.notes}
				<Notes text={t.notes} class="rounded-[18px] bg-well px-4 py-3.5 text-[15px] font-semibold dark:bg-surface" />
			{/if}

			{#if drill}
				<section class="flex flex-col gap-2">
					<h2 class="px-1 text-[15px] font-bold">Drill</h2>
					<a
						href="/exercises/{drill.next.exercise}?from={encodeURIComponent(`/templates/${t.id}`)}"
						class="flex items-center gap-3 rounded-3xl bg-surface p-3.5 shadow-card"
					>
						<span class="flex size-10 shrink-0 items-center justify-center rounded-[13px] bg-tint text-on-tint">
							<Icon name="repeat" size="1.25rem" />
						</span>
						<span class="min-w-0 flex-1">
							<span class="block text-[15px] font-bold">Try {drill.next.name}</span>
							<span class="block text-xs font-semibold text-ink-2">Last time {drill.last.name}</span>
						</span>
						<span class="text-ink-3"><Icon name="chevron-right" size="1rem" stroke={2.2} /></span>
					</a>
				</section>
			{/if}

			{#if loads.length}
				<section class="flex flex-col gap-2">
					<h2 class="px-1 text-[15px] font-bold">Last time</h2>
					<div class="rounded-3xl bg-surface py-0.5 shadow-card">
						{#each shown('loads', loads) as l (l.exercise)}
							<p class="flex min-h-[52px] items-center gap-3 px-4 [&:not(:first-child)]:shadow-[inset_0_1px_0_var(--line)]">
								<span class="min-w-0 flex-1 truncate text-[15px] font-bold">{l.name}</span>
								<span class="shrink-0 font-[family-name:var(--font-digits)] text-lg font-bold">{l.text}</span>
							</p>
						{/each}
						{@render more('loads', loads.length)}
					</div>
				</section>
			{/if}

			{#if note && data.last}
				<section class="flex flex-col gap-2">
					<h2 class="px-1 text-[15px] font-bold">Last note</h2>
					<a href="/history/{data.last.id}" class="rounded-3xl bg-surface p-4 shadow-card">
						<p class="line-clamp-3 text-[15px] leading-[1.4] font-semibold">{note}</p>
						<p class="mt-2 text-xs font-semibold text-ink-2">{day(data.last.local_date)}</p>
					</a>
				</section>
			{/if}

			{#if data.runs.length}
				<section class="flex flex-col gap-2">
					<h2 class="px-1 text-[15px] font-bold">Runs</h2>
					<div class="rounded-3xl bg-surface py-0.5 shadow-card">
						{#each shown('runs', data.runs) as r (r.id)}
							<a href="/history/{r.id}" class="flex min-h-[52px] items-center gap-3 px-4 [&:not(:first-child)]:shadow-[inset_0_1px_0_var(--line)]">
								<span class="flex-1 text-[15px] font-bold">{day(r.local_date)}</span>
								{#if r.elapsed_seconds}
									<span class="text-xs font-semibold text-ink-2">{Math.max(1, Math.round(r.elapsed_seconds / 60))} min</span>
								{/if}
								<span class="text-ink-3"><Icon name="chevron-right" size="1rem" stroke={2.2} /></span>
							</a>
						{/each}
						{@render more('runs', data.runs.length)}
					</div>
				</section>
			{:else if t.sections.length}
				<!-- Until a first run fills the Overview, it shows what the session holds. -->
				<section class="rounded-3xl bg-surface px-[18px] py-1 shadow-card">
					{#each t.sections as section, i (i)}
						{@const choice = only(section)}
						<a
							href="/templates/{t.id}?tab=exercises&open={i}"
							data-sveltekit-replacestate
							class="flex min-h-[76px] items-center gap-4 py-2.5 [&:not(:first-child)]:shadow-[inset_0_1px_0_var(--line)]"
						>
							<span class="w-[34px] shrink-0 font-[family-name:var(--font-digits)] text-[34px] leading-none font-bold text-ink-3 opacity-60">
								{String(i + 1).padStart(2, '0')}
							</span>
							<span class="min-w-0 flex-1">
								<span class="flex flex-wrap items-center gap-x-2 gap-y-1">
									<span class="text-base font-bold break-words">{section.name}</span>
									{#if choice}
										<span class="rounded-full bg-tint px-2.5 py-0.5 text-xs font-bold text-on-tint">{pickLabel(choice)}</span>
									{/if}
								</span>
								{#if firstNames(section) && firstNames(section) !== section.name}
									<span class="mt-0.5 block truncate text-xs font-semibold text-ink-2">{firstNames(section)}</span>
								{/if}
							</span>
							<span class="text-ink-3"><Icon name="chevron-right" size="1rem" stroke={2.2} /></span>
						</a>
					{/each}
				</section>
				<p class="px-1 text-xs font-semibold text-ink-2">Not run yet. Your runs, loads and notes show here after the first one.</p>
			{/if}

			<section class="flex flex-col gap-2">
				<h2 class="px-1 text-[15px] font-bold">Where you can do it</h2>
				{#if centres.length}
					<div class="flex flex-wrap gap-2">
						{#each centres as c, i (c.id)}
							{@const on = c.sessions.includes(t.id)}
							<button
								type="button"
								class="h-11 max-w-full truncate rounded-full px-4 text-[15px] font-bold {on ? 'bg-ink text-ground' : 'bg-surface text-ink shadow-card-sm'}"
								aria-pressed={on}
								onclick={() => toggleCentre(i)}
								use:haptic
							>
								{c.name}
							</button>
						{/each}
					</div>
				{:else}
					<a href="/profile" class="flex min-h-11 items-center px-1 text-xs font-semibold text-ink-2">Add your climbing centres in Profile</a>
				{/if}
			</section>
		{:else}
			{#each t.sections as section, i (i)}
				<details class="group rounded-3xl bg-surface shadow-card" open={t.sections.length === 1 || i === opened}>
					<summary class="flex min-h-[68px] cursor-pointer list-none items-center gap-3.5 px-4 py-3 [&::-webkit-details-marker]:hidden">
						<span class="flex size-[30px] shrink-0 items-center justify-center rounded-full bg-ink text-[15px] font-bold text-ground">{i + 1}</span>
						<span class="min-w-0 flex-1">
							<span class="block text-[17px] font-bold break-words">{section.name}</span>
							{#if only(section)}
								<span class="mt-1 inline-block rounded-full bg-tint px-2.5 py-0.5 text-xs font-bold text-on-tint">{pickLabel(only(section)!)}</span>
							{:else}
								<span class="block text-xs font-semibold text-ink-2">
									{steps(section)} exercise{steps(section) === 1 ? '' : 's'}
								</span>
							{/if}
						</span>
						<span class="flex size-9 shrink-0 items-center justify-center rounded-full bg-well text-ink-2 transition-transform group-open:rotate-180">
							<Icon name="chevron-down" size="1.125rem" stroke={2.4} />
						</span>
					</summary>
					<div class="flex flex-col gap-2 px-4 pb-4">
						{#if section.notes}
							<Notes text={section.notes} class="px-1 text-xs font-semibold text-ink-2" />
						{/if}
						{#each section.items as item, j (j)}
							{#if item.step}
								{@render row(item.step.exercise, item.step.name, stepMeta(item.step))}
							{:else}
								<div class="mt-1 flex flex-col gap-2">
									{#if !only(section)}
										<div class="flex flex-wrap items-center gap-2 px-1">
											<span class="text-[15px] font-bold">{item.choice.name}</span>
											<span class="rounded-full bg-tint px-2.5 py-1 text-xs font-bold text-on-tint">{pickLabel(item.choice)}</span>
										</div>
									{/if}
									{#if item.choice.notes}
										<p class="line-clamp-2 px-1 text-xs font-semibold text-ink-2">{plainText(item.choice.notes)}</p>
									{/if}
									<div class="rounded-[18px] p-1 shadow-[inset_0_0_0_1.5px_color-mix(in_srgb,var(--tint)_60%,transparent)]">
										{#each item.choice.options as option, k (k)}
											{@render row(option.exercise, option.name, stepMeta(option))}
										{/each}
									</div>
								</div>
							{/if}
						{:else}
							<p class="px-1 text-xs font-semibold text-ink-2">No exercises</p>
						{/each}
					</div>
				</details>
			{:else}
				<p class="rounded-3xl bg-surface p-[18px] text-[15px] font-semibold text-ink-2 shadow-card">No sections yet.</p>
			{/each}
		{/if}

		<div class="sticky bottom-[calc(var(--above-bar)-0.75rem)] z-20 -mx-4 mt-auto bg-ground/85 px-4 py-3 backdrop-blur-md">
			<Button disabled={doing !== null} onclick={start}>
				<Icon name="play" />
				{doing === 'start' ? 'Starting…' : 'Start session'}
			</Button>
		</div>
	</div>
</div>

{#snippet more(key: string, count: number)}
	{#if count > 3 && !unfolded.includes(key)}
		<button
			type="button"
			class="flex min-h-11 w-full items-center justify-center gap-1.5 text-[15px] font-bold text-ink-2 shadow-[inset_0_1px_0_var(--line)]"
			onclick={() => (unfolded = [...unfolded, key])}
		>
			+{count - 3} more<Icon name="chevron-down" size="1rem" stroke={2.4} />
		</button>
	{/if}
{/snippet}

{#snippet row(exercise: string, name: string, meta: string)}
	<a
		href="/exercises/{exercise}?from={encodeURIComponent(`/templates/${t.id}?tab=exercises`)}"
		class="flex min-h-[52px] items-center gap-3 rounded-[14px] px-3 py-2 [&:not(:first-child)]:shadow-[inset_0_1px_0_var(--line)]"
	>
		<span class="min-w-0 flex-1">
			<span class="block text-[15px] font-bold break-words">{name}</span>
			<span class="block text-xs font-semibold text-ink-2">{meta}</span>
		</span>
		<span class="text-ink-3"><Icon name="chevron-right" size="1rem" stroke={2.2} /></span>
	</a>
{/snippet}
