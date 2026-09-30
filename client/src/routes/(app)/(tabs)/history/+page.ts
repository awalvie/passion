import { request } from '$lib/api';
import type { RunSummary } from '$lib/run';

// An unfinished run counts for nothing, so History leaves it out.
export async function load() {
	const { runs } = await request<{ runs: RunSummary[] }>('GET', '/api/v1/runs');
	return { runs: runs.filter((r) => r.finished_at !== null) };
}
