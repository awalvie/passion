import { request, unreachable } from '$lib/api';
import { addDays, mondayOf } from '$lib/dates';
import { loadGoals } from '$lib/goals';
import { loadToday, type Cycle, type ScheduledDay, type TimeOff } from '$lib/plan';
import type { Repeat } from '$lib/repeat';
import { loadTemplates } from '$lib/template';

export async function load({ url }) {
	try {
		const today = await loadToday();
		// Six whole weeks: by default this week and the five after it.
		// The calendar can show six other weeks, from the Monday in ?from.
		const asked = url.searchParams.get('from') ?? '';
		const from = /^\d{4}-\d{2}-\d{2}$/.test(asked) && !isNaN(Date.parse(asked)) ? mondayOf(asked) : mondayOf(today);
		const [{ days }, { cycles }, { repeats }, { time_off: timeOff }, templates, goals] = await Promise.all([
			request<{ days: ScheduledDay[] }>('GET', `/api/v1/scheduled-sessions?from=${from}&to=${addDays(from, 41)}`),
			request<{ cycles: Cycle[] }>('GET', '/api/v1/cycles'),
			request<{ repeats: Repeat[] }>('GET', '/api/v1/repeats'),
			request<{ time_off: TimeOff[] }>('GET', '/api/v1/time-off'),
			loadTemplates(),
			url.searchParams.get('view') === 'goals' ? loadGoals() : []
		]);
		return { today, from, days, cycles, repeats, timeOff, templates, goals, offline: false };
	} catch (e) {
		if (!unreachable(e)) throw e;
		return { today: '', from: '', days: [], cycles: [], repeats: [], timeOff: [], templates: [], goals: [], offline: true };
	}
}
