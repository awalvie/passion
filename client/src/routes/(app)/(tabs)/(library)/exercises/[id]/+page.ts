import { notFound, request } from '$lib/api';
import { loadExercises, sourcesOf, type Exercise } from '$lib/exercise';
import { loadGrades, type Scale } from '$lib/grades';
import type { HistorySession } from '$lib/run';

export async function load({ params }) {
	// The exercise still opens without its history, which needs both answers.
	const history = Promise.all([
		request<{ sessions: HistorySession[] }>('GET', `/api/v1/exercises/${params.id}/history`),
		loadGrades()
	]).then(
		([h, g]) => ({ sessions: h.sessions, scales: g.grades.scales }),
		() => ({ sessions: [] as HistorySession[], scales: [] as Scale[] })
	);
	try {
		const [exercise, exercises, { sessions, scales }] = await Promise.all([
			request<Exercise>('GET', `/api/v1/exercises/${params.id}`),
			loadExercises(),
			history
		]);
		return { exercise, sources: sourcesOf(exercises), sessions, scales };
	} catch (e) {
		notFound(e);
	}
}
