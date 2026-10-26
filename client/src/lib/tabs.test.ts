import { expect, test } from 'vitest';
import { tabOf, tabs, under } from './tabs';

test('under holds for the root itself and the pages below it, not for a longer name', () => {
	expect(under('/plan', '/plan')).toBe(true);
	expect(under('/plan/cycles/new', '/plan')).toBe(true);
	expect(under('/planet', '/plan')).toBe(false);
	expect(under('/pla', '/plan')).toBe(false);
});

test('only Today itself is under the root of Today', () => {
	expect(under('/', '/')).toBe(true);
	expect(under('/plan', '/')).toBe(false);
});

test.each([
	['/', 'Today'],
	['/plan', 'Plan'],
	['/plan/cycles/abc', 'Plan'],
	['/journal', 'Journal'],
	['/journal/new', 'Journal'],
	['/templates', 'Library'],
	['/templates/abc', 'Library'],
	['/exercises/new', 'Library'],
	['/history', 'History'],
	['/history/progress', 'History'],
	['/history/exercise/abc', 'History']
])('%s belongs to the %s tab', (path, label) => {
	expect(tabs[tabOf(path)].label).toBe(label);
});

// The rail and the tab bar draw Profile apart from the tabs, and a run page is no tab's.
test.each(['/profile', '/run/abc', '/run/abc/finish', '/login', '/exercisesx'])('%s belongs to no tab', (path) => {
	expect(tabOf(path)).toBe(-1);
});
