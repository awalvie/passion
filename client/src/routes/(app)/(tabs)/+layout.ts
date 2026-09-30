import { request, unreachable } from '$lib/api';
import type { RunSummary } from '$lib/run';
import { storedRuns } from '$lib/runState.svelte';

// Reading url makes this run on every tab change, so the live bar goes as soon
// as a session ends. With no signal, it offers the run this phone holds.
export async function load({ url }) {
	void url.pathname;
	let runs: RunSummary[];
	try {
		runs = (await request<{ runs: RunSummary[] }>('GET', '/api/v1/runs')).runs;
	} catch (e) {
		if (!unreachable(e)) throw e;
		runs = storedRuns();
	}
	const live = runs
		.filter((r) => r.finished_at === null)
		.sort((a, b) => b.started_at.localeCompare(a.started_at))[0];
	return { live: live ?? null };
}
