import { request, unreachable } from '$lib/api';
import { isStale, type RunSummary } from '$lib/run';
import { storedRuns } from '$lib/runState.svelte';

// Reading url makes this run on every tab change, so the live bar goes as soon
// as a session ends. With no signal, it offers the run this phone holds, and
// runs is null. A run left open from an earlier day is stale, not live.
export async function load({ url }) {
	void url.pathname;
	let runs: RunSummary[] | null = null;
	try {
		runs = (await request<{ runs: RunSummary[] }>('GET', '/api/v1/runs')).runs;
	} catch (e) {
		if (!unreachable(e)) throw e;
	}
	const open = (runs ?? storedRuns())
		.filter((r) => r.finished_at === null)
		.sort((a, b) => b.started_at.localeCompare(a.started_at));
	const stale = open.filter((r) => isStale(r));
	return { live: open.find((r) => !stale.includes(r)) ?? null, stale, runs };
}
