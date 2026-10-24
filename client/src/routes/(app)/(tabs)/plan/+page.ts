import { request, unreachable } from '$lib/api';
import { addDays, mondayOf } from '$lib/dates';
import { loadToday, type Cycle, type ScheduledDay } from '$lib/plan';
import { loadTemplates } from '$lib/template';

export async function load() {
	try {
		const today = await loadToday();
		// Whole weeks: last week, this week and the four after it.
		const monday = mondayOf(today);
		const [{ days }, { cycles }, templates] = await Promise.all([
			request<{ days: ScheduledDay[] }>(
				'GET',
				`/api/v1/scheduled-sessions?from=${addDays(monday, -7)}&to=${addDays(monday, 34)}`
			),
			request<{ cycles: Cycle[] }>('GET', '/api/v1/cycles'),
			loadTemplates()
		]);
		return { today, days, cycles, templates, offline: false };
	} catch (e) {
		if (!unreachable(e)) throw e;
		return { today: '', days: [], cycles: [], templates: [], offline: true };
	}
}
