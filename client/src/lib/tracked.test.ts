import { beforeEach, expect, test, vi } from 'vitest';
import type { HistorySession, LoggedSet } from './run';
import { bestWeight, track, tracked } from './tracked';

vi.mock('$app/navigation', () => ({ goto: vi.fn() }));

const set = (weight_kg: number | null): LoggedSet => ({
	step: 's1',
	number: 1,
	exercise: 'ex-dl',
	reps: 5,
	seconds: null,
	weight_kg
});

const session = (local_date: string, weights: (number | null)[]): HistorySession => ({
	run: `run-${local_date}`,
	name: 'Strength',
	local_date,
	sets: weights.map(set),
	climbs: []
});

function serve(sessions: HistorySession[]) {
	vi.stubGlobal(
		'fetch',
		vi.fn(async (url: string) =>
			url.endsWith('/history') ? Response.json({ sessions }) : Response.json({ id: 'ex-dl', name: 'Deadlift' })
		)
	);
}

beforeEach(() => {
	const items = new Map<string, string>();
	vi.stubGlobal('localStorage', {
		getItem: (k: string) => items.get(k) ?? null,
		setItem: (k: string, v: string) => items.set(k, v),
		removeItem: (k: string) => items.delete(k)
	});
});

test('tracked is empty before anything is tracked', () => {
	expect(tracked()).toStrictEqual([]);
});

test('track puts the newest choice first and keeps each exercise once', () => {
	track('a');
	track('b');
	track('a');
	expect(tracked()).toStrictEqual(['a', 'b']);
});

test('bestWeight is the heaviest set from the first day to the last, both included', async () => {
	serve([
		session('2026-09-27', [200]),
		session('2026-09-28', [100, 120]),
		session('2026-10-02', [null, 110]),
		session('2026-10-04', [130]),
		session('2026-10-05', [300])
	]);
	expect(await bestWeight('ex-dl', '2026-09-28', '2026-10-04')).toStrictEqual({ name: 'Deadlift', kg: 130 });
	expect(fetch).toHaveBeenCalledWith('/api/v1/exercises/ex-dl', expect.anything());
	expect(fetch).toHaveBeenCalledWith('/api/v1/exercises/ex-dl/history', expect.anything());
});

test('bestWeight has no weight when no set in the days carries one', async () => {
	serve([session('2026-09-29', [null]), session('2026-10-10', [80])]);
	expect(await bestWeight('ex-dl', '2026-09-28', '2026-10-04')).toStrictEqual({ name: 'Deadlift', kg: null });
});
