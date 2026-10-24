import { loadExercises, sourcesOf } from '$lib/exercise';

export async function load() {
	return { sources: sourcesOf(await loadExercises()) };
}
