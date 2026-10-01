import { request, unreachable } from '$lib/api';
import { addDays, mondayOf } from '$lib/dates';
import { loadToday, type Cycle, type ScheduledDay } from '$lib/plan';
import type { SessionTemplate } from '$lib/template';

export async function load() {
	try {
		const today = await loadToday();
		// Whole weeks: last week, this week and the four after it.
		const monday = mondayOf(today);
		const [{ days }, { cycles }, { session_templates }] = await Promise.all([
			request<{ days: ScheduledDay[] }>(
				'GET',
				`/api/v1/scheduled-sessions?from=${addDays(monday, -7)}&to=${addDays(monday, 34)}`
			),
			request<{ cycles: Cycle[] }>('GET', '/api/v1/cycles'),
			request<{ session_templates: SessionTemplate[] }>('GET', '/api/v1/session-templates')
		]);
		return { today, days, cycles, templates: session_templates, offline: false };
	} catch (e) {
		if (!unreachable(e)) throw e;
		return { today: '', days: [], cycles: [], templates: [], offline: true };
	}
}
