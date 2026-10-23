import { request } from '$lib/api';
import { loadGrades } from '$lib/grades';
import { loadToday } from '$lib/plan';
import { buckets, type DatedClimb } from '$lib/progress';
import type { RunSummary } from '$lib/run';

// An unfinished run counts for nothing, so History leaves it out.
export async function load() {
	const today = await loadToday();
	const weeks = buckets('10w', today);
	const [{ runs }, { climbs }, { grades }] = await Promise.all([
		request<{ runs: RunSummary[] }>('GET', '/api/v1/runs'),
		request<{ climbs: DatedClimb[] }>('GET', `/api/v1/climbs?from=${weeks[0].from}&to=${today}`),
		loadGrades()
	]);
	return { runs: runs.filter((r) => r.finished_at !== null), climbs, scales: grades.scales, weeks };
}
