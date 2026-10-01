// @ts-nocheck: node runs this with `node --test`, and the client has no node types.
import assert from 'node:assert/strict';
import { test } from 'node:test';
import { addDays, cycleWeek, mondayOf, weekday } from './dates.ts';

test('mondayOf finds the Monday on or before a date', () => {
	assert.equal(mondayOf('2026-10-01'), '2026-09-28');
	assert.equal(mondayOf('2026-09-28'), '2026-09-28');
	assert.equal(mondayOf('2026-10-04'), '2026-09-28');
});

test('cycleWeek counts whole weeks from the start', () => {
	assert.deepEqual(cycleWeek('2026-10-05', '2026-11-01', '2026-10-05'), { week: 1, of: 4 });
	assert.deepEqual(cycleWeek('2026-10-05', '2026-11-01', '2026-10-18'), { week: 2, of: 4 });
	assert.deepEqual(cycleWeek('2026-10-05', '2026-11-02', '2026-11-02'), { week: 5, of: 5 });
});

test('addDays crosses months, years and leap days', () => {
	assert.equal(addDays('2026-01-31', 1), '2026-02-01');
	assert.equal(addDays('2026-12-31', 1), '2027-01-01');
	assert.equal(addDays('2028-02-28', 1), '2028-02-29');
	assert.equal(addDays('2026-03-01', -1), '2026-02-28');
	assert.equal(addDays('2026-10-01', 28), '2026-10-29');
});

test('addDays keeps the day across a daylight saving change', () => {
	assert.equal(addDays('2026-03-28', 1), '2026-03-29');
	assert.equal(addDays('2026-03-29', 1), '2026-03-30');
});

test('weekday names the day the date falls on', () => {
	assert.equal(weekday('2026-10-01'), 'Thu');
	assert.equal(weekday('2026-10-05'), 'Mon');
});
