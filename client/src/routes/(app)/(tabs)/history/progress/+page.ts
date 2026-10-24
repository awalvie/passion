import { request } from '$lib/api';
import { loadGrades } from '$lib/grades';
import { loadToday, type Cycle } from '$lib/plan';
import { buckets, readSpan, type DatedClimb } from '$lib/progress';
import type { RunSummary } from '$lib/run';

const views = ['time', 'climbs', 'grade'] as const;
export type View = (typeof views)[number];

export async function load({ url }) {
	const view = (views as readonly string[]).includes(url.searchParams.get('view') ?? '') ? (url.searchParams.get('view') as View) : 'time';
	const [today, { cycles }] = await Promise.all([loadToday(), request<{ cycles: Cycle[] }>('GET', '/api/v1/cycles')]);
	const span = readSpan(url.searchParams.get('range'), today, cycles);
	const chart = buckets(span, today);
	const to = span.to < today ? span.to : today;
	const [{ runs }, { climbs }, { grades }] = await Promise.all([
		request<{ runs: RunSummary[] }>('GET', '/api/v1/runs'),
		request<{ climbs: DatedClimb[] }>('GET', `/api/v1/climbs?from=${span.from}&to=${to}`),
		loadGrades()
	]);
	const done = runs.filter((r) => r.finished_at !== null && r.local_date >= span.from && r.local_date <= to);
	return {
		view,
		today,
		span,
		...chart,
		// The cycles the sheet offers: those begun, the latest first.
		cycles: cycles.filter((c) => c.starts <= today).sort((a, b) => b.starts.localeCompare(a.starts)),
		running: cycles.find((c) => c.starts <= today && today <= c.ends) ?? null,
		runs: done,
		climbs,
		scales: grades.scales
	};
}
