import { beforeEach, expect, test, vi } from 'vitest';

type Leaving = { type: string; cancel: () => void };
let before: (n: Leaving) => void = () => {};

vi.mock('$app/navigation', () => ({
	beforeNavigate: (fn: (n: Leaving) => void) => {
		before = fn;
	}
}));

import { guardUnsaved } from './unsaved';

const leave = (type: string) => ({ type, cancel: vi.fn() });
const confirmWith = (answer: boolean) => {
	const confirm = vi.fn(() => answer);
	vi.stubGlobal('confirm', confirm);
	return confirm;
};

beforeEach(() => {
	vi.unstubAllGlobals();
	before = () => {};
});

test('guardUnsaved lets a page with nothing changed go without asking', () => {
	const confirm = confirmWith(false);
	guardUnsaved(() => false);
	for (const type of ['link', 'goto', 'popstate', 'leave']) {
		const n = leave(type);
		before(n);
		expect(n.cancel, type).not.toHaveBeenCalled();
	}
	expect(confirm).not.toHaveBeenCalled();
});

test('guardUnsaved asks before an in-app move, and stays when the answer is no', () => {
	const confirm = confirmWith(false);
	guardUnsaved(() => true);
	for (const type of ['link', 'goto', 'popstate']) {
		const n = leave(type);
		before(n);
		expect(n.cancel, type).toHaveBeenCalledOnce();
	}
	expect(confirm).toHaveBeenCalledTimes(3);
});

test('guardUnsaved lets an in-app move go on once the answer is yes', () => {
	confirmWith(true);
	guardUnsaved(() => true);
	const n = leave('link');
	before(n);
	expect(n.cancel).not.toHaveBeenCalled();
});

// A reload or a closed tab cannot show our own prompt, so cancel hands it to
// the browser's.
test('guardUnsaved leaves a reload or a closed tab to the browser prompt', () => {
	const confirm = confirmWith(true);
	guardUnsaved(() => true);
	const n = leave('leave');
	before(n);
	expect(n.cancel).toHaveBeenCalledOnce();
	expect(confirm).not.toHaveBeenCalled();
});

test('guardUnsaved reads the page as it is when the move starts, not when it was set up', () => {
	confirmWith(false);
	let changed = false;
	guardUnsaved(() => changed);
	const first = leave('link');
	before(first);
	expect(first.cancel).not.toHaveBeenCalled();

	changed = true;
	const second = leave('link');
	before(second);
	expect(second.cancel).toHaveBeenCalledOnce();

	// A saved page marks itself as leaving before it goes.
	changed = false;
	const third = leave('link');
	before(third);
	expect(third.cancel).not.toHaveBeenCalled();
});
