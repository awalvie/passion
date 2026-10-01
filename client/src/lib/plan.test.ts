import { expect, test } from 'vitest';
import { plannedDays } from './plan';

const cycle = {
	starts: '2026-10-01',
	ends: '2026-10-10',
	block_days: 4,
	days: [
		{ day: 3, template: 'b' },
		{ day: 1, template: 'a' }
	]
};

test('plannedDays repeats the block until the cycle ends', () => {
	expect(plannedDays(cycle, '2026-09-01')).toStrictEqual([
		{ date: '2026-10-01', template: 'a' },
		{ date: '2026-10-03', template: 'b' },
		{ date: '2026-10-05', template: 'a' },
		{ date: '2026-10-07', template: 'b' },
		{ date: '2026-10-09', template: 'a' }
	]);
});

test('plannedDays leaves out days already past', () => {
	expect(plannedDays(cycle, '2026-10-05').map((d) => d.date)).toStrictEqual(['2026-10-05', '2026-10-07', '2026-10-09']);
});

test('plannedDays drops days beyond the block', () => {
	expect(plannedDays({ ...cycle, block_days: 2 }, '2026-09-01').map((d) => d.template)).toStrictEqual(['a', 'a', 'a', 'a', 'a']);
});

test('plannedDays plans nothing while a date is empty', () => {
	expect(plannedDays({ ...cycle, starts: '' }, '2026-09-01')).toStrictEqual([]);
});
