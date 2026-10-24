import { loadExercises } from '$lib/exercise';

export async function load() {
	return { exercises: await loadExercises() };
}
