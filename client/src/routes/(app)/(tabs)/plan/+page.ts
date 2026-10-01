import { request, unreachable } from '$lib/api';
import { addDays } from '$lib/dates';
import { loadToday, type Cycle, type ScheduledDay } from '$lib/plan';
import type { SessionTemplate } from '$lib/template';

export async function load() {
	try {
		const today = await loadToday();
		const [{ days }, { cycles }, { session_templates }] = await Promise.all([
			request<{ days: ScheduledDay[] }>(
				'GET',
				`/api/v1/scheduled-sessions?from=${addDays(today, -7)}&to=${addDays(today, 27)}`
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
