import { notFound, request } from '$lib/api';
import { loadExercises, sourcesOf, type Exercise } from '$lib/exercise';

export async function load({ params }) {
	try {
		const [exercise, exercises] = await Promise.all([request<Exercise>('GET', `/api/v1/exercises/${params.id}`), loadExercises()]);
		return { exercise, sources: sourcesOf(exercises) };
	} catch (e) {
		notFound(e);
	}
}
