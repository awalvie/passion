import type { HandleClientError } from '@sveltejs/kit';
import { RequestFailed } from '$lib/api';

// A page that cannot reach the server says so, in place of "Internal Error".
// A fetch with no signal fails with a TypeError, its words set by the browser;
// a proxy whose server cannot be reached answers 502 to 504.
export const handleError: HandleClientError = ({ error, message }) => {
	const lost =
		!navigator.onLine ||
		(error instanceof RequestFailed && error.status >= 502 && error.status <= 504) ||
		(error instanceof TypeError && /failed to fetch|load failed|networkerror/i.test(error.message));
	return { message: lost ? 'No signal' : message };
};
