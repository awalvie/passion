import { request } from '$lib/api';
import type { Exercise } from '$lib/exercise';

export async function load() {
	const { exercises } = await request<{ exercises: Exercise[] }>('GET', '/api/v1/exercises');
	return { exercises };
}
