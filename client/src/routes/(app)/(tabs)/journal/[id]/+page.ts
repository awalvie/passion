import { error } from '@sveltejs/kit';
import { loadJournal } from '$lib/journal';

// The page shows the entry's week, so it loads the whole journal and finds
// the entry in it.
export async function load({ params }) {
	const journal = await loadJournal();
	const entry = journal.entries.find((e) => e.id === params.id);
	if (!entry) error(404, 'Not found');
	return { entry, journal };
}
