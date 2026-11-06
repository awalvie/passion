import { expect, test, vi } from 'vitest';
import { clearDrafts } from './session';

class MemoryStorage {
	[k: string]: unknown;
	getItem(k: string): string | null {
		return Object.hasOwn(this, k) ? String(this[k]) : null;
	}
	setItem(k: string, v: string) {
		this[k] = String(v);
	}
	removeItem(k: string) {
		delete this[k];
	}
}

test('signing out drops unsaved journal text and keeps the phone settings', () => {
	vi.stubGlobal('localStorage', new MemoryStorage());
	localStorage.setItem('passion-journal-draft:new', '{"body":"Sore finger"}');
	localStorage.setItem('passion-journal-draft:e-1', '{"body":"Edit"}');
	localStorage.setItem('passion-theme', 'dark');

	clearDrafts();

	expect(Object.keys(localStorage)).toStrictEqual(['passion-theme']);
});
