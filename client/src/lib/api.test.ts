import { isHttpError } from '@sveltejs/kit';
import { expect, test, vi } from 'vitest';
import { notFound, RequestFailed } from './api';

vi.mock('$app/navigation', () => ({ goto: vi.fn() }));

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
