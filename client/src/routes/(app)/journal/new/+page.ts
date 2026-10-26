import { loadToday } from '$lib/plan';

export async function load() {
	return { today: await loadToday() };
}
