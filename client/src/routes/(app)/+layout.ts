import { redirect } from '@sveltejs/kit';
import { token } from '$lib/session';

// Every page in this group needs an account. The API checks the token on each
// request; this only saves a signed-out visitor a screen of errors. The front
// door shows Welcome; a deep link or the Home Screen app goes to Log in.
export function load({ url }) {
	if (token()) return;
	const installed =
		matchMedia('(display-mode: standalone)').matches || (navigator as Navigator & { standalone?: boolean }).standalone === true;
	redirect(307, url.pathname === '/' && !installed ? '/welcome' : '/login');
}
