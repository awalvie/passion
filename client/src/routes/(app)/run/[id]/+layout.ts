import { notFound } from '$lib/api';
import { openRun } from '$lib/runState.svelte';

export async function load({ params }) {
	try {
		await openRun.load(params.id);
	} catch (e) {
		notFound(e);
	}
}
