import { redirect } from '@sveltejs/kit';
import { notFound, request } from '$lib/api';
import { loadExercises, sourcesOf } from '$lib/exercise';
import type { SessionTemplate } from '$lib/template';

export async function load({ params }) {
	try {
		const [template, exercises] = await Promise.all([
			request<SessionTemplate>('GET', `/api/v1/session-templates/${params.id}`),
			loadExercises()
		]);
		if (template.shipped || template.retired_at) redirect(307, `/templates/${params.id}`);
		return { template, exercises, sources: sourcesOf(exercises) };
	} catch (e) {
		notFound(e);
	}
}
