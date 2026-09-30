import { error } from '@sveltejs/kit';
import { RequestFailed, request } from '$lib/api';
import type { Run } from '$lib/run';

export async function load({ params }) {
	try {
		return { run: await request<Run>('GET', `/api/v1/runs/${params.id}`) };
	} catch (e) {
		if (e instanceof RequestFailed && e.status === 404) error(404, 'Not found');
		throw e;
	}
}
