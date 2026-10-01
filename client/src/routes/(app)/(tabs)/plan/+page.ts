import { request, unreachable } from '$lib/api';
import { addDays } from '$lib/dates';
import { loadToday, type Cycle, type ScheduledDay } from '$lib/plan';

export async function load() {
	try {
		const today = await loadToday();
		const [{ days }, { cycles }] = await Promise.all([
			request<{ days: ScheduledDay[] }>(
				'GET',
				`/api/v1/scheduled-sessions?from=${addDays(today, -7)}&to=${addDays(today, 27)}`
			),
			request<{ cycles: Cycle[] }>('GET', '/api/v1/cycles')
		]);
		return { today, days, cycles, offline: false };
	} catch (e) {
		if (!unreachable(e)) throw e;
		return { today: '', days: [], cycles: [], offline: true };
	}
}
