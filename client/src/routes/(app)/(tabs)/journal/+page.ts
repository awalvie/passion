import { loadJournal } from '$lib/journal';
import { loadToday } from '$lib/plan';

// Today is for the date picker of New entry, which a laptop opens beside the
// list.
export async function load() {
	const [journal, today] = await Promise.all([loadJournal(), loadToday()]);
	return { journal, today };
}
