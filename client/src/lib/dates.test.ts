import { expect, test } from 'vitest';
import { addDays, cycleWeek, gridWeeks, mondayOf, weekday, yearSpan } from './dates';

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

test('gridWeeks covers the span in whole weeks', () => {
	const weeks = gridWeeks('2026-09-16', '2026-10-05');
	expect(weeks.map((w) => [w[0], w[6]])).toStrictEqual([
		['2026-09-14', '2026-09-20'],
		['2026-09-21', '2026-09-27'],
		['2026-09-28', '2026-10-04'],
		['2026-10-05', '2026-10-11']
	]);
	expect(gridWeeks('2026-09-14', '2026-09-14')).toHaveLength(1);
});

test('yearSpan clips a span to the year', () => {
	expect(yearSpan('2026-01-01', '2026-12-31', 2026)).toStrictEqual({ left: 0, width: 1 });
	expect(yearSpan('2025-12-01', '2026-01-01', 2026)).toStrictEqual({ left: 0, width: 1 / 365 });
	expect(yearSpan('2026-07-02', '2026-07-02', 2026)?.left).toBeCloseTo(182 / 365);
	expect(yearSpan('2025-01-01', '2025-12-31', 2026)).toBeNull();
});

test('weekday names the day the date falls on', () => {
	expect(weekday('2026-10-01')).toBe('Thu');
	expect(weekday('2026-10-05')).toBe('Mon');
});
