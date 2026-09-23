import { redirect } from '@sveltejs/kit';
import { token } from '$lib/session';

// Every page in this group needs an account. The API checks the token on each
// request; this only saves a signed-out visitor a screen of errors.
export function load() {
	if (!token()) redirect(307, '/login');
}
