import { request } from '$lib/api';
import type { RunSummary } from '$lib/run';

// Reading url makes this run on every tab change, so the live bar goes as soon
// as a session ends.
export async function load({ url }) {
	void url.pathname;
	const { runs } = await request<{ runs: RunSummary[] }>('GET', '/api/v1/runs');
	const live = runs
		.filter((r) => r.finished_at === null)
		.sort((a, b) => b.started_at.localeCompare(a.started_at))[0];
	return { live: live ?? null };
}
