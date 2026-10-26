import { notFound, request } from '$lib/api';
import type { JournalEntry } from '$lib/journal';

export async function load({ params }) {
	const entry = await request<JournalEntry>('GET', `/api/v1/journal/${params.id}`).catch(notFound);
	return { entry };
}
