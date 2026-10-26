import { request } from '$lib/api';
import { loadGrades } from '$lib/grades';
import { loadToday, type Cycle } from '$lib/plan';
import { buckets, readFilter, readSpan, trends, trendWeeks, type DatedClimb } from '$lib/progress';
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
	const weeks = trendWeeks(today);
	const [{ climbs }, { grades }, recent] = await Promise.all([
		request<{ climbs: DatedClimb[] }>('GET', `/api/v1/climbs?from=${span.from}&to=${to}`),
		loadGrades(),
		request<{ climbs: DatedClimb[] }>('GET', `/api/v1/climbs?from=${weeks[0].from}&to=${weeks[7].to}`)
	]);
	const done = finished.filter((r) => r.local_date >= span.from && r.local_date <= to);
	const ran = finished.filter((r) => r.local_date >= weeks[0].from && r.local_date <= weeks[7].to);
	return {
		view,
		today,
		first,
		fresh: !finished.length,
		span,
		filter: readFilter(url.searchParams),
		...chart,
		// The cycles the sheet offers: those begun, the latest first.
		cycles: cycles.filter((c) => c.starts <= today).sort((a, b) => b.starts.localeCompare(a.starts)),
		running: cycles.find((c) => c.starts <= today && today <= c.ends) ?? null,
		runs: done,
		climbs,
		// Nothing in the eight weeks before this one leaves no trend to show.
		trends: ran.length ? trends(ran, recent.climbs, weeks, first) : null,
		scales: grades.scales
	};
}
