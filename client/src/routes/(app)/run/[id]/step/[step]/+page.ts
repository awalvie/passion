import { error } from '@sveltejs/kit';
import { openRun } from '$lib/runState.svelte';

export async function load({ params, parent }) {
	await parent();
	if (!openRun.step(params.step)) error(404, 'Not found');
}
