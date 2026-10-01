import { request } from '$lib/api';
import { addDays } from '$lib/dates';
import { newId } from '$lib/id';
import { loadToday, type Cycle } from '$lib/plan';
import type { SessionTemplate } from '$lib/template';

export async function load() {
	const [today, { session_templates }] = await Promise.all([
		loadToday(),
		request<{ session_templates: SessionTemplate[] }>('GET', '/api/v1/session-templates')
	]);
	const cycle: Cycle = { id: newId(), name: '', starts: today, ends: addDays(today, 27), block_days: 7, days: [] };
	return { cycle, templates: session_templates };
}
