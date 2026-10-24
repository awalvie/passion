import { expect, test } from 'vitest';
import { formatDuration, formatMinutes } from './exercise';

test('formatDuration leaves out the parts that are zero', () => {
	expect(formatDuration(0)).toBe('0s');
	expect(formatDuration(45)).toBe('45s');
	expect(formatDuration(3600)).toBe('1h');
	expect(formatDuration(5430)).toBe('1h 30m 30s');
});

test('formatMinutes rounds to the nearest minute and never shows less than one', () => {
	expect(formatMinutes(0)).toBe('1m');
	expect(formatMinutes(29)).toBe('1m');
	expect(formatMinutes(89)).toBe('1m');
	expect(formatMinutes(90)).toBe('2m');
	expect(formatMinutes(5430)).toBe('1h 31m');
});
