import { request, unreachable } from '$lib/api';
import { loadCentres } from '$lib/centres';
import { addDays, mondayOf } from '$lib/dates';
import { loadToday, localToday, type Cycle, type ScheduledDay } from '$lib/plan';
import { openRun } from '$lib/runState.svelte';
import { loadTemplates, type SessionTemplate } from '$lib/template';

export async function load({ parent }) {
	// The card shows the live session from the run itself, which works offline
	// from the copy on the phone.
	const live = parent().then(({ live }) => live && openRun.load(live.id));
	try {
		const today = await loadToday();
		const monday = mondayOf(today);
		const sunday = addDays(monday, 6);
		const [{ days }, cycles, library] = await Promise.all([
			// Next week too, so on a Sunday the next session still shows.
			request<{ days: ScheduledDay[] }>(
				'GET',
				`/api/v1/scheduled-sessions?from=${monday}&to=${addDays(monday, 13)}`
			),
			// The week and the card still show without the cycle names.
			request<{ cycles: Cycle[] }>('GET', '/api/v1/cycles').then(
				(r) => r.cycles,
				() => [] as Cycle[]
			),
			// Today still shows without Your sessions.
			loadTemplates().catch(() => [] as SessionTemplate[])
		]);
		const week = days.filter((d) => d.local_date <= sunday);
		const next =
			days
				.filter((d) => d.local_date > today && d.status === 'planned')
				.sort((a, b) => a.local_date.localeCompare(b.local_date))[0] ?? null;
		// Only someone who has logged nothing yet sees the first-week card.
		const { runs } = await parent();
		const [templates, centres] = await Promise.all([
			Promise.all(
				// The whole week's, so a tap on another day needs no load.
				[...new Set([...week, ...(next ? [next] : [])].map((d) => d.template))].map((id) =>
					request<SessionTemplate>('GET', `/api/v1/session-templates/${id}`)
				)
			),
			runs?.length === 0 ? loadCentres().then((c) => c.length) : null
		]);
		await live;
		return {
			today,
			monday,
			week,
			next,
			cycles,
			templates: new Map(templates.map((t) => [t.id, t])),
			library,
			centres,
			offline: false
		};
	} catch (e) {
		// With no signal, Today still opens, and the card leads to the live run.
		if (!unreachable(e)) throw e;
		const today = localToday(Intl.DateTimeFormat().resolvedOptions().timeZone);
		await live;
		return {
			today,
			monday: mondayOf(today),
			week: [] as ScheduledDay[],
			next: null,
			cycles: [] as Cycle[],
			templates: new Map<string, SessionTemplate>(),
			library: [] as SessionTemplate[],
			centres: null,
			offline: true
		};
	}
}
