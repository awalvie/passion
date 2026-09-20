import { token } from './session';

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

export async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
	const headers: Record<string, string> = {};
	if (body !== undefined) headers['Content-Type'] = 'application/json';

	const bearer = token();
	if (bearer) headers['Authorization'] = `Bearer ${bearer}`;

	const res = await fetch(path, {
		method,
		headers,
		body: body === undefined ? undefined : JSON.stringify(body)
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
		throw new RequestFailed(res.status, detail);
	}

	return res.status === 204 ? (undefined as T) : res.json();
}
