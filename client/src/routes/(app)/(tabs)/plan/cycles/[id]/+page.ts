import { request } from '$lib/api';
import { addDays, mondayOf } from '$lib/dates';
import { loadToday, type Cycle, type ScheduledDay, type TimeOff } from '$lib/plan';
import type { Repeat } from '$lib/repeat';
import { loadTemplates } from '$lib/template';

export async function load({ params }) {
	const [today, cycle, { cycles }, templates, { repeats }, { time_off: timeOff }] = await Promise.all([
		loadToday(),
		request<Cycle>('GET', `/api/v1/cycles/${params.id}`),
		request<{ cycles: Cycle[] }>('GET', '/api/v1/cycles'),
		loadTemplates(),
		request<{ repeats: Repeat[] }>('GET', '/api/v1/repeats'),
		request<{ time_off: TimeOff[] }>('GET', '/api/v1/time-off')
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
		templates,
		// Every row in the cycle's dates, so a day another cycle or a one-off
		// holds does not look free.
		days: days.filter((d) => d.local_date >= cycle.starts && d.local_date <= cycle.ends),
		cycleNames: Object.fromEntries(cycles.map((c) => [c.id, c.name])),
		repeats,
		// The time off that falls in the cycle's dates.
		timeOff: timeOff.filter((o) => o.starts <= cycle.ends && (!o.ends || o.ends >= cycle.starts)),
		week: days.filter((d) => d.local_date >= monday && d.local_date <= addDays(monday, 6))
	};
}
