import { request, unreachable } from '$lib/api';
import { addDays, mondayOf } from '$lib/dates';
import { loadToday, localToday, type Cycle, type ScheduledDay } from '$lib/plan';
import type { SessionTemplate } from '$lib/template';

export async function load() {
	try {
		const today = await loadToday();
		const monday = mondayOf(today);
		const [{ days: week }, cycles] = await Promise.all([
			request<{ days: ScheduledDay[] }>(
				'GET',
				`/api/v1/scheduled-sessions?from=${monday}&to=${addDays(monday, 6)}`
			),
			// The week and the card still show without the cycle names.
			request<{ cycles: Cycle[] }>('GET', '/api/v1/cycles').then(
				(r) => r.cycles,
				() => [] as Cycle[]
			)
		]);
		const days = week.filter((d) => d.local_date === today);
		const templates = await Promise.all(
			[...new Set(days.map((d) => d.template))].map((id) =>
				request<SessionTemplate>('GET', `/api/v1/session-templates/${id}`)
			)
		);
		return {
			today,
			monday,
			week,
			days,
			cycles,
			templates: new Map(templates.map((t) => [t.id, t])),
			offline: false
		};
	} catch (e) {
		// With no signal, Today still opens, and the live bar leads to the run.
		if (!unreachable(e)) throw e;
		const today = localToday(Intl.DateTimeFormat().resolvedOptions().timeZone);
		return {
			today,
			monday: mondayOf(today),
			week: [] as ScheduledDay[],
			days: [] as ScheduledDay[],
			cycles: [] as Cycle[],
			templates: new Map<string, SessionTemplate>(),
			offline: true
		};
	}
}
