import { loadExercises, sourcesOf } from '$lib/exercise';

export async function load() {
	const exercises = await loadExercises();
	return { exercises, sources: sourcesOf(exercises) };
}
