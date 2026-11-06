import { request } from '$lib/api';
import { loadGrades } from '$lib/grades';
import { loadToday } from '$lib/plan';
import { buckets, tenWeeks, type DatedClimb, type DatedSet } from '$lib/progress';
import type { RunSummary } from '$lib/run';

// An unfinished run counts for nothing, so History leaves it out.
export async function load() {
	const today = await loadToday();
	const weeks = buckets(tenWeeks(today), today).buckets;
	const [{ runs }, { climbs }, { grades }] = await Promise.all([
		request<{ runs: RunSummary[] }>('GET', '/api/v1/runs'),
		request<{ climbs: DatedClimb[] }>('GET', `/api/v1/climbs?from=${weeks[0].from}&to=${today}`),
		loadGrades()
	]);
	// What each row says happened comes after the list; until then, or if it
	// fails, a row shows its length. A run logged in a zone ahead of the
	// account's can be dated after today.
	const from = runs.at(-1)?.local_date ?? today;
	const to = runs[0] && runs[0].local_date > today ? runs[0].local_date : today;
	const done = Promise.all([
		request<{ climbs: DatedClimb[] }>('GET', `/api/v1/climbs?from=${from}&to=${to}`),
		request<{ sets: DatedSet[] }>('GET', `/api/v1/sets?from=${from}&to=${to}`)
	]).then(
		([c, s]) => ({ climbs: c.climbs, sets: s.sets }),
		() => null
	);
	return { runs: runs.filter((r) => r.finished_at !== null), climbs, scales: grades.scales, weeks, done };
}
