import { notFound, request } from '$lib/api';
import type { Run } from '$lib/run';

export async function load({ params }) {
	try {
		return { run: await request<Run>('GET', `/api/v1/runs/${params.id}`) };
	} catch (e) {
		notFound(e);
	}
}
