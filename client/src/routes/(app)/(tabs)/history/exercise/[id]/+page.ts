import { request, RequestFailed } from '$lib/api';
import { loadGrades } from '$lib/grades';
import type { HistorySession } from '$lib/run';

export async function load({ params, url }) {
	const [{ sessions }, exercise, { grades }] = await Promise.all([
		request<{ sessions: HistorySession[] }>('GET', `/api/v1/exercises/${params.id}/history`),
		// An exercise typed into a run has no library page, only its history.
		request<{ name: string }>('GET', `/api/v1/exercises/${params.id}`).catch((e) => {
			if (e instanceof RequestFailed && e.status === 404) return null;
			throw e;
		}),
		loadGrades()
	]);
	return { sessions, name: exercise?.name ?? url.searchParams.get('name') ?? 'Exercise', scales: grades.scales };
}
