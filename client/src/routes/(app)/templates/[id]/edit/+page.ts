import { error, redirect } from '@sveltejs/kit';
import { RequestFailed, request } from '$lib/api';
import { sourcesOf, type Exercise } from '$lib/exercise';
import type { SessionTemplate } from '$lib/template';

export async function load({ params }) {
	try {
		const [template, { exercises }] = await Promise.all([
			request<SessionTemplate>('GET', `/api/v1/session-templates/${params.id}`),
			request<{ exercises: Exercise[] }>('GET', '/api/v1/exercises')
		]);
		if (template.shipped || template.retired_at) redirect(307, `/templates/${params.id}`);
		return { template, sources: sourcesOf(exercises) };
	} catch (e) {
		if (e instanceof RequestFailed && e.status === 404) error(404, 'Not found');
		throw e;
	}
}
