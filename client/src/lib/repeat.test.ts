import { expect, test } from 'vitest';
import serverCases from '../../../server/db/testdata/repeat_days.json';
import { nthDay, repeatDays, ruleText, type RepeatRule } from './repeat';

// The server's own cases, so the preview and the server agree.
const cases = serverCases as (RepeatRule & { name: string; from: string; until: string; days: string[] })[];

test.each(cases)('repeatDays: $name', (c) => {
	expect(repeatDays(c, c.from, c.until)).toStrictEqual(c.days);
});

test('nthDay counts the days the rule falls on', () => {
	const tueThu: RepeatRule = { starts: '2026-10-27', ends: null, every: 1, unit: 'week', weekdays: [2, 4] };
	expect(nthDay(tueThu, 1)).toBe('2026-10-27');
	expect(nthDay(tueThu, 12)).toBe('2026-12-03');
	expect(nthDay({ ...tueThu, every: 3, unit: 'day', weekdays: [] }, 3)).toBe('2026-11-02');
	expect(nthDay({ ...tueThu, starts: '2026-10-29', every: 2 }, 2)).toBe('2026-11-10');
	expect(nthDay({ ...tueThu, weekdays: [] }, 1)).toBeNull();
});

test('ruleText names the days', () => {
	expect(ruleText({ every: 1, unit: 'week', weekdays: [4, 2] })).toBe('every Tuesday and Thursday');
	expect(ruleText({ every: 2, unit: 'week', weekdays: [1, 3, 5] })).toBe('every 2 weeks on Monday, Wednesday and Friday');
	expect(ruleText({ every: 1, unit: 'day', weekdays: [] })).toBe('every day');
	expect(ruleText({ every: 3, unit: 'day', weekdays: [] })).toBe('every 3 days');
});
