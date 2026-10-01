import { request } from '$lib/api';
import { addDays, mondayOf } from '$lib/dates';
import { loadToday, type Cycle, type ScheduledDay } from '$lib/plan';
import type { SessionTemplate } from '$lib/template';

export async function load({ params }) {
	const [today, cycle, { cycles }, { session_templates }] = await Promise.all([
		loadToday(),
		request<Cycle>('GET', `/api/v1/cycles/${params.id}`),
		request<{ cycles: Cycle[] }>('GET', '/api/v1/cycles'),
		request<{ session_templates: SessionTemplate[] }>('GET', '/api/v1/session-templates')
	]);
	// One call covers the cycle and this week, which may lie outside it.
	const monday = mondayOf(today);
	const from = cycle.starts < monday ? cycle.starts : monday;
	const to = cycle.ends > addDays(monday, 6) ? cycle.ends : addDays(monday, 6);
	const { days } = await request<{ days: ScheduledDay[] }>('GET', `/api/v1/scheduled-sessions?from=${from}&to=${to}`);
	return {
		today,
		monday,
		cycle,
		templates: session_templates,
		// Every row in the cycle's dates, so a day another cycle or a one-off
		// holds does not look free.
		days: days.filter((d) => d.local_date >= cycle.starts && d.local_date <= cycle.ends),
		cycleNames: Object.fromEntries(cycles.map((c) => [c.id, c.name])),
		week: days.filter((d) => d.local_date >= monday && d.local_date <= addDays(monday, 6))
	};
}
