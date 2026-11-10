import { request } from '$lib/api';
import { addDays } from '$lib/dates';
import { loadToday } from '$lib/plan';
import type { Run, RunSummary } from '$lib/run';

const day = (v: string | null) => (v && /^\d{4}-\d{2}-\d{2}$/.test(v) ? v : null);

// The finished sessions between two days, oldest first, each in full, as a
// coach reads a log. Without days it is the last 4 weeks.
export async function load({ url }) {
	const today = await loadToday();
	const to = day(url.searchParams.get('to')) ?? today;
	const asked = day(url.searchParams.get('from'));
	const from = asked && asked <= to ? asked : addDays(to, -27);
	const { runs } = await request<{ runs: RunSummary[] }>('GET', '/api/v1/runs');
	const picked = runs.filter((r) => r.finished_at !== null && r.local_date >= from && r.local_date <= to).reverse();
	const full = await Promise.all(picked.map((r) => request<Run>('GET', `/api/v1/runs/${r.id}`)));
	return { today, from, to, runs: full };
}
