import { expect, test } from 'vitest';
import { plainText, plural } from './text';

test('plainText drops emphasis, code and link markers', () => {
	expect(plainText('**Setup** - Stand tall')).toBe('Setup - Stand tall');
	expect(plainText('a *slow* and __firm__ pull')).toBe('a slow and firm pull');
	expect(plainText('use `20mm` or [this edge](https://x.y/e)')).toBe('use 20mm or this edge');
	expect(plainText('## How to\n\nHang')).toBe('How to\n\nHang');
});

test('plainText keeps list markers and words with stars or underscores', () => {
	expect(plainText('- one\n* two\n1. three')).toBe('- one\n* two\n1. three');
	expect(plainText('3 * 4 and snake_case_name')).toBe('3 * 4 and snake_case_name');
});

test('plural adds an s to every count but one', () => {
	expect(plural(0, 'set')).toBe('0 sets');
	expect(plural(1, 'set')).toBe('1 set');
	expect(plural(2, 'set')).toBe('2 sets');
});
