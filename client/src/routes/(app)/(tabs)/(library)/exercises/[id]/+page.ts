import { error } from '@sveltejs/kit';
import { RequestFailed, request } from '$lib/api';
import { sourcesOf, type Exercise } from '$lib/exercise';

export async function load({ params }) {
	try {
		const [exercise, { exercises }] = await Promise.all([
			request<Exercise>('GET', `/api/v1/exercises/${params.id}`),
			request<{ exercises: Exercise[] }>('GET', '/api/v1/exercises')
		]);
		return { exercise, sources: sourcesOf(exercises) };
	} catch (e) {
		if (e instanceof RequestFailed && e.status === 404) error(404, 'Not found');
		throw e;
	}
}
