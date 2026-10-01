import { error } from '@sveltejs/kit';
import { RequestFailed, request } from '$lib/api';
import { loadCentres } from '$lib/centres';
import type { SessionTemplate } from '$lib/template';

export async function load({ params }) {
	try {
		const [template, centres] = await Promise.all([
			request<SessionTemplate>('GET', `/api/v1/session-templates/${params.id}`),
			loadCentres()
		]);
		return { template, centres };
	} catch (e) {
		if (e instanceof RequestFailed && e.status === 404) error(404, 'Not found');
		throw e;
	}
}
