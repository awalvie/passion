import { redirect } from '@sveltejs/kit';
import { token } from '$lib/session';

export function load() {
	if (token()) redirect(307, '/');
}
