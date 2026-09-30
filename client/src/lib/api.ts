import { goto } from '$app/navigation';
import { clearToken, token } from './session';

// The one shape every failure comes back in. See server/api/errors.go.
export type APIError = {
	code: string;
	message: string;
	fields?: Record<string, string>;
};

export class RequestFailed extends Error {
	constructor(
		readonly status: number,
		readonly error: APIError
	) {
		super(error.message);
	}
}

export async function request<T>(
	method: string,
	path: string,
	body?: unknown,
	signal?: AbortSignal
): Promise<T> {
	const headers: Record<string, string> = {};
	if (body !== undefined) headers['Content-Type'] = 'application/json';

	const bearer = token();
	if (bearer) headers['Authorization'] = `Bearer ${bearer}`;

	const res = await fetch(path, {
		method,
		headers,
		body: body === undefined ? undefined : JSON.stringify(body),
		signal
	});

	if (!res.ok) {
		// A proxy or a crash can answer with something that is not our error
		// shape, so a failure to parse must not hide the status.
		let detail: APIError = { code: 'unknown', message: `Request failed (${res.status}).` };
		try {
			detail = (await res.json()).error ?? detail;
		} catch {
			/* keep the fallback */
		}
		// A token that expired or was signed out elsewhere. A wrong password at
		// sign-in answers with another code, so it stays on its own page.
		if (res.status === 401 && detail.code === 'unauthenticated') {
			clearToken();
			await goto('/login');
		}
		throw new RequestFailed(res.status, detail);
	}

	return res.status === 204 ? (undefined as T) : res.json();
}

// unreachable says a request never got an answer from Passion: no signal, or
// a proxy in front of it answering for a server it cannot reach.
export function unreachable(e: unknown): boolean {
	return !(e instanceof RequestFailed) || e.status >= 500;
}

// describe turns a failed request into one line for a form, with one sentence
// per field that the server named.
export function describe(e: unknown, label: (field: string) => string): string {
	if (!(e instanceof RequestFailed)) return 'Could not reach the server. Try again.';
	const fields = e.error.fields;
	if (fields) {
		return Object.entries(fields)
			.map(([field, problem]) => `${label(field)} ${problem}.`)
			.join(' ');
	}
	return e.error.message;
}
