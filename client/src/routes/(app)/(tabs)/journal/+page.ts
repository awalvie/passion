import { loadJournal } from '$lib/journal';

export async function load() {
	return { journal: await loadJournal() };
}
