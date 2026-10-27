import { redirect } from '@sveltejs/kit';
import { notFound, request } from '$lib/api';
import type { Run } from '$lib/run';

// A laptop shows a session beside History's list.
export async function load({ params }) {
	if (matchMedia('(min-width: 64rem)').matches) redirect(307, `/history?open=${params.id}`);
	try {
		return { run: await request<Run>('GET', `/api/v1/runs/${params.id}`) };
	} catch (e) {
		notFound(e);
	}
}
