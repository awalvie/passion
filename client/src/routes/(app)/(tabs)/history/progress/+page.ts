import { request } from '$lib/api';
import { loadGrades } from '$lib/grades';
import { loadToday } from '$lib/plan';
import { buckets, ranges, type DatedClimb, type Range } from '$lib/progress';
import type { RunSummary } from '$lib/run';

const views = ['time', 'climbs', 'grade'] as const;
export type View = (typeof views)[number];

export async function load({ url }) {
	const view = (views as readonly string[]).includes(url.searchParams.get('view') ?? '') ? (url.searchParams.get('view') as View) : 'time';
	const range = ranges.some((r) => r.range === url.searchParams.get('range')) ? (url.searchParams.get('range') as Range) : '10w';
	const today = await loadToday();
	const bs = buckets(range, today);
	const [{ runs }, { climbs }, { grades }] = await Promise.all([
		request<{ runs: RunSummary[] }>('GET', '/api/v1/runs'),
		request<{ climbs: DatedClimb[] }>('GET', `/api/v1/climbs?from=${bs[0].from}&to=${today}`),
		loadGrades()
	]);
	const done = runs.filter((r) => r.finished_at !== null && r.local_date >= bs[0].from);
	return { view, range, buckets: bs, runs: done, climbs, scales: grades.scales };
}
