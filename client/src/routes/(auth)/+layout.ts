import { redirect } from '@sveltejs/kit';
import { token } from '$lib/session';

// Someone signed in has no use for these pages. A token the server no longer
// takes is cleared by the first request Today makes, which comes back here.
export function load() {
	if (token()) redirect(307, '/');
}
