import { error } from '@sveltejs/kit';
import { RequestFailed } from '$lib/api';
import { openRun } from '$lib/runState.svelte';

export async function load({ params }) {
	try {
		await openRun.load(params.id);
	} catch (e) {
		if (e instanceof RequestFailed && e.status === 404) error(404, 'Not found');
		throw e;
	}
}
