import { request } from '$lib/api';
import { loadGrades } from '$lib/grades';
import { loadToday, type Cycle } from '$lib/plan';
import { buckets, readFilter, readSpan, type DatedClimb } from '$lib/progress';
import type { RunSummary } from '$lib/run';

const views = ['time', 'climbs', 'grade'] as const;
export type View = (typeof views)[number];

export async function load({ url }) {
	const view = (views as readonly string[]).includes(url.searchParams.get('view') ?? '') ? (url.searchParams.get('view') as View) : 'time';
	const [today, { cycles }, { runs }] = await Promise.all([
		loadToday(),
		request<{ cycles: Cycle[] }>('GET', '/api/v1/cycles'),
		request<{ runs: RunSummary[] }>('GET', '/api/v1/runs')
	]);
	const finished = runs.filter((r) => r.finished_at !== null);
	const first = finished.reduce((d, r) => (r.local_date < d ? r.local_date : d), today);
	const span = readSpan(url.searchParams.get('range'), today, cycles, first);
	const chart = buckets(span, today);
	const to = span.to < today ? span.to : today;
	const [{ climbs }, { grades }] = await Promise.all([
		request<{ climbs: DatedClimb[] }>('GET', `/api/v1/climbs?from=${span.from}&to=${to}`),
		loadGrades()
	]);
	const done = finished.filter((r) => r.local_date >= span.from && r.local_date <= to);
	return {
		view,
		today,
		first,
		span,
		filter: readFilter(url.searchParams),
		...chart,
		// The cycles the sheet offers: those begun, the latest first.
		cycles: cycles.filter((c) => c.starts <= today).sort((a, b) => b.starts.localeCompare(a.starts)),
		running: cycles.find((c) => c.starts <= today && today <= c.ends) ?? null,
		runs: done,
		climbs,
		scales: grades.scales
	};
}
