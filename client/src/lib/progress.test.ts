import { expect, test } from 'vitest';
import type { Scale } from './grades';
import { bestSet, climbMarks, figure, headline, measure, setMarks, setValue, topSend } from './progress';
import type { Climb, HistorySession, SetFields } from './run';

const set = (s: Partial<SetFields>) => ({ reps: null, seconds: null, rep_seconds: null, weight_kg: null, ...s });
const session = (sets: SetFields[], climbs: Climb[] = []) =>
	({ run: 'r', name: 'S', local_date: '2026-10-01', sets, climbs }) as unknown as HistorySession;
const scale = { system: 'font', grades: ['6a', '6b', '6c', '7a'] } as Scale;
const climb = (grade: string, sent: boolean) => ({ grade, grade_system: 'font', sent }) as Climb;

test('measure follows weight, then time under load, then reps', () => {
	expect(measure([session([set({ reps: 5, weight_kg: 10 }), set({ reps: 5 })])])).toBe('kg');
	expect(measure([session([set({ reps: 5, seconds: 10 })])])).toBe('s');
	expect(measure([session([set({ reps: 3, rep_seconds: [3, 6, 9] })])])).toBe('s');
	expect(measure([session([set({ seconds: 45 })])])).toBe('s');
	expect(measure([session([set({ reps: 8 })])])).toBe('reps');
});

test('setValue sums a ladder and multiplies timed reps', () => {
	expect(setValue(set({ reps: 3, rep_seconds: [3, 6, 9] }), 's')).toBe(18);
	expect(setValue(set({ reps: 5, seconds: 10 }), 's')).toBe(50);
	expect(setValue(set({ seconds: 45 }), 's')).toBe(45);
	expect(setValue(set({ reps: 5, weight_kg: -20 }), 'kg')).toBe(-20);
	expect(setValue(set({ reps: 5 }), 'kg')).toBe(0);
	expect(setValue(set({ reps: 8 }), 'reps')).toBe(8);
});

test('bestSet takes the biggest value, then the most reps', () => {
	const sets = [set({ reps: 5, weight_kg: 10 }), set({ reps: 6, weight_kg: 10 }), set({ reps: 8, weight_kg: 7.5 })];
	expect(bestSet(sets, 'kg')).toBe(sets[1]);
	const assisted = [set({ reps: 5, weight_kg: -20 }), set({ reps: 5, weight_kg: -10 })];
	expect(bestSet(assisted, 'kg')).toBe(assisted[1]);
	expect(bestSet([], 'kg')).toBeNull();
});

test('setMarks leads with the best set and drops repeats of a value', () => {
	const sets = [set({ reps: 5, weight_kg: 10 }), set({ reps: 5, weight_kg: 10 }), set({ reps: 5, weight_kg: 7.5 })];
	expect(setMarks(sets, 2, 'kg')).toEqual([
		{ i: 2, v: 10, best: true },
		{ i: 2, v: 7.5 }
	]);
	expect(setMarks([], 0, 'kg')).toEqual([]);
});

test('climbMarks marks one best send and leaves unsent climbs hollow', () => {
	const climbs = [climb('6c', true), climb('6c', true), climb('7a', false), climb('5c', true)];
	expect(climbMarks(climbs, 1, scale)).toEqual([
		{ i: 1, v: 2, best: true, hollow: false },
		{ i: 1, v: 2, best: false, hollow: false },
		{ i: 1, v: 3, best: false, hollow: true }
	]);
	expect(climbMarks([climb('7a', false)], 0, scale)).toEqual([{ i: 0, v: 3, best: false, hollow: true }]);
});

test('topSend is the hardest sent climb on the scale', () => {
	expect(topSend([climb('6b', true), climb('7a', false), climb('6c', true)], scale)?.grade).toBe('6c');
	expect(topSend([climb('7a', false)], scale)).toBeNull();
});

test('headline compares the last two sessions that have a best set', () => {
	const now = session([set({ reps: 5, weight_kg: 12.5 })]);
	const empty = session([]);
	const then = session([set({ reps: 5, weight_kg: 10 })]);
	const h = headline([empty, now, empty, then], 'kg', null)!;
	expect(h.session).toBe(now);
	expect(h.main).toBe('5 × 12.5');
	expect(h.change).toBe(2.5);
	expect(headline([empty], 'kg', null)).toBeNull();
	expect(headline([now], 'kg', null)!.change).toBeNull();
});

test('headline gives a ladder its seconds and a send its grade', () => {
	const h = headline([session([set({ reps: 3, rep_seconds: [3, 6, 9], weight_kg: 10 })])], 'kg', null)!;
	expect([h.main, h.small]).toEqual(['10', 'kg · 3/6/9 s']);
	const t = headline([session([set({ reps: 5, seconds: 10 })])], 's', null)!;
	expect([t.main, t.small]).toEqual(['5 × 10', 's']);
	const g = headline([session([], [climb('6c', true)]), session([], [climb('6a', true)])], 'reps', scale)!;
	expect([g.main, g.change]).toEqual(['6c', 2]);
});

test('measure keeps reps when reps-only and timed sets are mixed', () => {
	expect(measure([session([set({ reps: 8 })]), session([set({ reps: 5, seconds: 10 })])])).toBe('reps');
});

test('a set with nothing in it is never the best', () => {
	const empty = set({});
	const skipped = set({ reps: 0, seconds: 10 });
	const real = set({ reps: 3 });
	expect(bestSet([empty, skipped], 'reps')).toBeNull();
	expect(bestSet([empty, real], 'reps')).toBe(real);
	expect(setMarks([empty, real], 0, 'reps')).toEqual([{ i: 0, v: 3, best: true }]);
	const h = headline([session([empty]), session([real])], 'reps', null)!;
	expect(h.main).toBe('3');
});

test('figure keeps a weighted hold its seconds in a row', () => {
	const hold = set({ seconds: 45, weight_kg: 10 });
	expect(figure(hold, 'kg', true)).toEqual(['10', 'kg · 45 s']);
	expect(figure(set({ reps: 3, rep_seconds: [3, 6, 9], weight_kg: 10 }), 'kg', true)).toEqual(['10', 'kg']);
	expect(figure(set({ seconds: 90 }), 's')).toEqual(['1:30', 'min']);
	expect(figure(set({ reps: 5, weight_kg: -20 }), 'kg')).toEqual(['5 × -20', 'kg']);
});
