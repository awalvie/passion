import { notFound, request } from '$lib/api';
import type { JournalEntry } from '$lib/journal';
import { loadToday } from '$lib/plan';

export async function load({ params }) {
	const [entry, today] = await Promise.all([
		request<JournalEntry>('GET', `/api/v1/journal/${params.id}`).catch(notFound),
		loadToday()
	]);
	return { entry, today };
}
