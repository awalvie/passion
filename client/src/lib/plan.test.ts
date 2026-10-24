import { expect, test, vi } from 'vitest';
import { plannedDays, saveCycle } from './plan';

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

test('saveCycle drops the days past the block and sends the rest', async () => {
	const fetch = vi.fn(async (_url: string, init: RequestInit) => Response.json({ ...JSON.parse(init.body as string), left_out: [{}] }));
	vi.stubGlobal('fetch', fetch);
	vi.stubGlobal('localStorage', { getItem: () => null });
	const c = { id: 'c1', name: 'Base', starts: '2026-10-01', ends: '2026-10-28', block_days: 3, block_from: '2026-10-01', goals: [], notes: null };

	const { leftOut } = await saveCycle({ ...c, days: [{ day: 1, template: 'a' }, { day: 5, template: 'b' }] });

	expect(leftOut).toBe(1);
	const [url, init] = fetch.mock.calls[0];
	expect(url).toBe('/api/v1/cycles/c1');
	expect(JSON.parse(init.body as string)).toStrictEqual({ name: 'Base', starts: '2026-10-01', ends: '2026-10-28', block_days: 3, goals: [], notes: null, days: [{ day: 1, template: 'a' }] });
	vi.unstubAllGlobals();
});
