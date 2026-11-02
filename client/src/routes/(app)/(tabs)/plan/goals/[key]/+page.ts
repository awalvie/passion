import { error } from '@sveltejs/kit';
import { loadGoals, pastPeriods } from '$lib/goals';
import { loadToday } from '$lib/plan';

export async function load({ params }) {
	const [today, goals] = await Promise.all([loadToday(), loadGoals()]);
	const period = pastPeriods(goals, today).find((p) => p.key === params.key);
	if (!period) error(404, 'No such period');
	return { period };
}
