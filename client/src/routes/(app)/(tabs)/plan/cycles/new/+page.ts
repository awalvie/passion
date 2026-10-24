import { addDays } from '$lib/dates';
import { newId } from '$lib/id';
import { loadToday, type Cycle } from '$lib/plan';
import { loadTemplates } from '$lib/template';

export async function load() {
	const [today, templates] = await Promise.all([loadToday(), loadTemplates()]);
	const cycle: Cycle = { id: newId(), name: '', starts: today, ends: addDays(today, 27), block_days: 7, block_from: today, days: [], goals: [], notes: null };
	return { cycle, today, templates };
}
