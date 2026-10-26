import { isHttpError } from '@sveltejs/kit';
import { expect, test, vi } from 'vitest';
import { goto } from '$app/navigation';
import { notFound, request, RequestFailed } from './api';

vi.mock('$app/navigation', () => ({ goto: vi.fn() }));

const storage = (items: Record<string, string>) => ({
	getItem: (k: string) => items[k] ?? null,
	setItem: (k: string, v: string) => (items[k] = v),
	removeItem: (k: string) => delete items[k]
});

// Today's data is asked for while the (app) layout decides where a signed-out
// visitor goes, so a 401 without a token must leave that to the layout.
test.each([
	{ sent: 'old', code: 'unauthenticated', away: true },
	{ sent: null, code: 'unauthenticated', away: false },
	{ sent: 'old', code: 'invalid_credentials', away: false }
])('a 401 $code with token $sent goes to Log in: $away', async ({ sent, code, away }) => {
	const items: Record<string, string> = sent ? { 'passion-token': sent } : {};
	vi.stubGlobal('localStorage', storage(items));
	vi.stubGlobal('fetch', vi.fn(async () => Response.json({ error: { code, message: 'no' } }, { status: 401 })));
	vi.mocked(goto).mockClear();

	await expect(request('GET', `/api/v1/runs?${code}${sent}`)).rejects.toBeInstanceOf(RequestFailed);
	expect(vi.mocked(goto).mock.calls).toEqual(away ? [['/login']] : []);
	expect('passion-token' in items).toBe(!!sent && !away);
});

const thrown = (e: unknown) => {
	try {
		notFound(e);
	} catch (x) {
		return x;
	}
};

test('notFound turns the server 404 into the page 404 and passes anything else on', () => {
	const missing = thrown(new RequestFailed(404, { code: 'not_found', message: 'gone' }));
	expect(isHttpError(missing, 404)).toBe(true);

	const failed = new RequestFailed(500, { code: 'internal', message: 'boom' });
	expect(thrown(failed)).toBe(failed);
	const offline = new TypeError('fetch failed');
	expect(thrown(offline)).toBe(offline);
});
