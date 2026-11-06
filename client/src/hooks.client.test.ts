import { expect, test, vi } from 'vitest';
import { RequestFailed } from '$lib/api';
import { handleError } from './hooks.client';

vi.mock('$app/navigation', () => ({ goto: vi.fn() }));

const handle = (error: unknown, onLine = true) => {
	vi.stubGlobal('navigator', { onLine });
	const args = { error, event: {}, status: 500, message: 'Internal Error' } as unknown as Parameters<typeof handleError>[0];
	return (handleError(args) as { message: string }).message;
};

const failed = (status: number) => new RequestFailed(status, { code: 'x', message: 'server words' });

test.each([
	['Chrome with no signal', new TypeError('Failed to fetch')],
	['Safari with no signal', new TypeError('Load failed')],
	['Firefox with no signal', new TypeError('NetworkError when attempting to fetch resource.')],
	['a proxy that cannot reach the server, 502', failed(502)],
	['a proxy that cannot reach the server, 503', failed(503)],
	['a proxy that timed out, 504', failed(504)]
])('%s reads as No signal', (_, error) => {
	expect(handle(error)).toBe('No signal');
});

test('a browser that says it is offline reads as No signal, whatever failed', () => {
	expect(handle(new Error('anything'), false)).toBe('No signal');
});

test.each([
	['a refused request', failed(403)],
	['a server that crashed', failed(500)],
	['a server that cannot do it, 501', failed(501)],
	['a bug in the page', new TypeError("Cannot read properties of undefined (reading 'id')")],
	['a plain error', new Error('boom')]
])('%s keeps the framework message', (_, error) => {
	expect(handle(error)).toBe('Internal Error');
});

test('a TypeError that is not a network failure is not No signal', () => {
	expect(handle(new TypeError('data.fetch is not a function'))).toBe('Internal Error');
});
