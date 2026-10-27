import { loadExerciseHistory } from '$lib/run';

export function load({ params, url }) {
	return loadExerciseHistory(params.id, url.searchParams.get('name'));
}
