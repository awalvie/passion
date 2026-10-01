import { expect, test } from 'vitest';
import { addDays, cycleWeek, mondayOf, weekday } from './dates';

test('mondayOf finds the Monday on or before a date', () => {
	expect(mondayOf('2026-10-01')).toBe('2026-09-28');
	expect(mondayOf('2026-09-28')).toBe('2026-09-28');
	expect(mondayOf('2026-10-04')).toBe('2026-09-28');
});

test('cycleWeek counts whole weeks from the start', () => {
	expect(cycleWeek('2026-10-05', '2026-11-01', '2026-10-05')).toStrictEqual({ week: 1, of: 4 });
	expect(cycleWeek('2026-10-05', '2026-11-01', '2026-10-18')).toStrictEqual({ week: 2, of: 4 });
	expect(cycleWeek('2026-10-05', '2026-11-02', '2026-11-02')).toStrictEqual({ week: 5, of: 5 });
});

test('addDays crosses months, years and leap days', () => {
	expect(addDays('2026-01-31', 1)).toBe('2026-02-01');
	expect(addDays('2026-12-31', 1)).toBe('2027-01-01');
	expect(addDays('2028-02-28', 1)).toBe('2028-02-29');
	expect(addDays('2026-03-01', -1)).toBe('2026-02-28');
	expect(addDays('2026-10-01', 28)).toBe('2026-10-29');
});

test('addDays keeps the day across a daylight saving change', () => {
	expect(addDays('2026-03-28', 1)).toBe('2026-03-29');
	expect(addDays('2026-03-29', 1)).toBe('2026-03-30');
});

test('weekday names the day the date falls on', () => {
	expect(weekday('2026-10-01')).toBe('Thu');
	expect(weekday('2026-10-05')).toBe('Mon');
});
