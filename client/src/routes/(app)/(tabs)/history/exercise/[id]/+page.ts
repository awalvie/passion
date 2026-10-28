import { redirect } from '@sveltejs/kit';
import { progressFrom } from '$lib/progress';
import { loadExerciseHistory } from '$lib/run';

// A laptop shows an exercise's history beside History's list, still leading
// back to Progress when opened from there.
export function load({ params, url }) {
	const name = url.searchParams.get('name');
	const from = progressFrom(url.searchParams.get('from'));
	if (matchMedia('(min-width: 64rem)').matches)
		redirect(
			307,
			`/history?exercise=${params.id}${name ? `&name=${encodeURIComponent(name)}` : ''}${from ? `&from=${encodeURIComponent(from)}` : ''}`
		);
	return loadExerciseHistory(params.id, name);
}
