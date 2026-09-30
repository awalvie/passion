import { error } from '@sveltejs/kit';
import { RequestFailed, request } from '$lib/api';
import type { SessionTemplate } from '$lib/template';

export async function load({ params }) {
	try {
		return {
			template: await request<SessionTemplate>('GET', `/api/v1/session-templates/${params.id}`)
		};
	} catch (e) {
		if (e instanceof RequestFailed && e.status === 404) error(404, 'Not found');
		throw e;
	}
}
