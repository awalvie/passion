import { request, unreachable } from '$lib/api';
import { addDays, mondayOf } from '$lib/dates';
import { loadToday, type Cycle, type ScheduledDay } from '$lib/plan';
import { loadTemplates } from '$lib/template';

export async function load({ url }) {
	try {
		const today = await loadToday();
		// Six whole weeks: by default last week, this week and the four after it.
		// The calendar can show six other weeks, from the Monday in ?from.
		const asked = url.searchParams.get('from') ?? '';
		const from = /^\d{4}-\d{2}-\d{2}$/.test(asked) ? mondayOf(asked) : addDays(mondayOf(today), -7);
		const [{ days }, { cycles }, templates] = await Promise.all([
			request<{ days: ScheduledDay[] }>('GET', `/api/v1/scheduled-sessions?from=${from}&to=${addDays(from, 41)}`),
			request<{ cycles: Cycle[] }>('GET', '/api/v1/cycles'),
			loadTemplates()
		]);
		return { today, from, days, cycles, templates, offline: false };
	} catch (e) {
		if (!unreachable(e)) throw e;
		return { today: '', from: '', days: [], cycles: [], templates: [], offline: true };
	}
}
