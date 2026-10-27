import { redirect } from '@sveltejs/kit';
import { loadExerciseHistory } from '$lib/run';

// A laptop shows an exercise's history beside History's list.
export function load({ params, url }) {
	const name = url.searchParams.get('name');
	if (matchMedia('(min-width: 64rem)').matches)
		redirect(307, `/history?exercise=${params.id}${name ? `&name=${encodeURIComponent(name)}` : ''}`);
	return loadExerciseHistory(params.id, name);
}
