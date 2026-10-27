import { expect, test, vi } from 'vitest';
import { formatDuration, formatMinutes, loadExercises, matches, summary } from './exercise';

vi.mock('$app/navigation', () => ({ goto: vi.fn() }));

test('formatDuration leaves out the parts that are zero', () => {
	expect(formatDuration(0)).toBe('0s');
	expect(formatDuration(45)).toBe('45s');
	expect(formatDuration(3600)).toBe('1h');
	expect(formatDuration(5430)).toBe('1h 30m 30s');
});

test('summary counts one set or one rep in the singular', () => {
	const d = {
		kind: 'reps_and_sets' as const,
		per_side: false,
		sets: null,
		reps: null,
		rep_seconds: null,
		rep_rest_seconds: null,
		set_rest_seconds: null,
		prep_seconds: null,
		duration_seconds: null
	};
	expect(summary({ ...d, sets: 1 })).toBe('1 set');
	expect(summary({ ...d, reps: 1 })).toBe('1 rep');
	expect(summary({ ...d, reps: 8 })).toBe('8 reps');
});

test('formatMinutes rounds to the nearest minute and never shows less than one', () => {
	expect(formatMinutes(0)).toBe('1m');
	expect(formatMinutes(29)).toBe('1m');
	expect(formatMinutes(89)).toBe('1m');
	expect(formatMinutes(90)).toBe('2m');
	expect(formatMinutes(5430)).toBe('1h 31m');
});

test('matches reads the search and each filter, and lets everything through without them', () => {
	const r = { name: 'Density Hang', shipped: true, private: false, source: 'C4HP', tags: ['fingers'] };
	expect(matches(r, {})).toBe(true);
	expect(matches(r, { q: ' hang ' })).toBe(true);
	expect(matches(r, { q: 'pull' })).toBe(false);
	expect(matches(r, { origin: 'builtin', source: 'C4HP', tag: 'fingers' })).toBe(true);
	expect(matches(r, { origin: 'yours' })).toBe(false);
	expect(matches(r, { tag: 'core' })).toBe(false);
});

test('loadExercises reads the library list', async () => {
	const fetch = vi.fn(async () => Response.json({ exercises: [{ id: 'e1' }] }));
	vi.stubGlobal('fetch', fetch);
	vi.stubGlobal('localStorage', { getItem: () => null });
	expect(await loadExercises()).toStrictEqual([{ id: 'e1' }]);
	expect(fetch).toHaveBeenCalledWith('/api/v1/exercises', expect.objectContaining({ method: 'GET' }));
	vi.unstubAllGlobals();
});

test('loadExercises asks the server again when told to load fresh', async () => {
	const fetch = vi.fn(async () => Response.json({ exercises: [] }));
	vi.stubGlobal('fetch', fetch);
	vi.stubGlobal('localStorage', { getItem: () => null });
	await loadExercises(true);
	await loadExercises();
	expect(fetch).toHaveBeenCalledTimes(1);
	await loadExercises(true);
	expect(fetch).toHaveBeenCalledTimes(2);
	vi.unstubAllGlobals();
});
