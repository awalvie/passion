import { isRedirect } from '@sveltejs/kit';
import { expect, test, vi } from 'vitest';
import { load as appLoad } from '../routes/(app)/+layout';
import { load as welcomeLoad } from '../routes/welcome/+page';

function setUp(signedIn: boolean, installed: 'no' | 'display-mode' | 'navigator') {
	vi.stubGlobal('localStorage', { getItem: () => (signedIn ? 'tok' : null) });
	vi.stubGlobal('matchMedia', () => ({ matches: installed === 'display-mode' }));
	vi.stubGlobal('navigator', installed === 'navigator' ? { standalone: true } : {});
}

function goesTo(run: () => unknown) {
	try {
		run();
	} catch (e) {
		if (isRedirect(e)) return e.location;
		throw e;
	}
	return null;
}

test.each([
	{ path: '/', signedIn: false, installed: 'no', to: '/welcome' },
	{ path: '/run/x', signedIn: false, installed: 'no', to: '/login' },
	{ path: '/', signedIn: false, installed: 'display-mode', to: '/login' },
	{ path: '/', signedIn: false, installed: 'navigator', to: '/login' },
	{ path: '/', signedIn: true, installed: 'no', to: null }
] as const)('$path, signed in $signedIn, installed $installed, goes to $to', ({ path, signedIn, installed, to }) => {
	setUp(signedIn, installed);
	expect(goesTo(() => appLoad({ url: new URL(`http://x${path}`) } as never))).toBe(to);
});

test('the landing page sends someone signed in to Today', () => {
	setUp(true, 'no');
	expect(goesTo(welcomeLoad)).toBe('/');
	setUp(false, 'no');
	expect(goesTo(welcomeLoad)).toBe(null);
});
