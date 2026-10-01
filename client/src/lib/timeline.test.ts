import { expect, test } from 'vitest';
import {
	at,
	elapsed,
	endSet,
	newClock,
	rows,
	timeline,
	togglePause,
	type Clock,
	type TimedStep
} from './timeline';

const hang: TimedStep = {
	sets: 2,
	reps: 3,
	rep_seconds: 7,
	rep_rest_seconds: 3,
	set_rest_seconds: 60,
	prep_seconds: 5,
	per_side: false
};

const kinds = (s: TimedStep) => timeline(s).map((p) => `${p.kind}${p.ms / 1000}`);

test('a set is hangs with rests between, and sets have a longer rest between', () => {
	expect(kinds(hang)).toStrictEqual([
		'prep5',
		'hang7', 'rest3', 'hang7', 'rest3', 'hang7', 'rest60',
		'hang7', 'rest3', 'hang7', 'rest3', 'hang7'
	]);
});

test('per side does all reps on the left, then the right, as two blocks', () => {
	const p = timeline({ ...hang, sets: 1, reps: 2, per_side: true, prep_seconds: 0 });
	expect(p.map((x) => `${x.kind}${x.side}${x.block}`)).toStrictEqual([
		'hangLeft0',
		'restLeft0',
		'hangLeft0',
		'restLeft0',
		'hangRight1',
		'restRight1',
		'hangRight1'
	]);
});

test('a phase with no time is left out', () => {
	expect(kinds({ ...hang, sets: 1, reps: 2, prep_seconds: null, rep_rest_seconds: 0 })).toStrictEqual(['hang7', 'hang7']);
});

test('the clock leaves out pauses and adds skips', () => {
	const c: Clock = { startedAt: 1000, pausedAt: null, pausedMs: 500, skipMs: 2000 };
	expect(elapsed(c, 11_000)).toBe(11_500);
	expect(elapsed({ ...c, pausedAt: 6000 }, 99_000)).toBe(6500);
});

test('a pause stops the clock until it is resumed', () => {
	const paused = togglePause(newClock(0), 4000);
	expect(elapsed(paused, 60_000)).toBe(4000);
	expect(elapsed(togglePause(paused, 10_000), 11_000)).toBe(5000);
});

test('at finds the phase and the time left in it', () => {
	const p = timeline(hang);
	expect(at(p, 0)).toStrictEqual({ index: 0, left: 5000 });
	expect(at(p, 6000)).toStrictEqual({ index: 1, left: 6000 });
	expect(at(p, 10 ** 9).index).toBe(p.length);
});

test('rows count each finished block, and the one in progress only when asked', () => {
	const p = timeline(hang);
	expect(rows(p, 5000 + 7000, {})).toStrictEqual([]);
	expect(rows(p, 5000 + 7000, {}, true)).toStrictEqual([1]);
	expect(rows(p, 5000 + 27_000, {})).toStrictEqual([3]);
	expect(rows(p, 10 ** 9, {})).toStrictEqual([3, 3]);
});

test('end set during prep changes nothing', () => {
	const p = timeline(hang);
	const c: Clock = { startedAt: 0, pausedAt: null, pausedMs: 0, skipMs: 0 };
	expect(endSet(p, c, 1000, {})).toStrictEqual({ clock: c, short: {} });
});

test('end set keeps the hangs done and moves to the rest after the set', () => {
	const p = timeline(hang);
	const c: Clock = { startedAt: 0, pausedAt: null, pausedMs: 0, skipMs: 0 };
	const { clock, short } = endSet(p, c, 5000 + 7000 + 1000, {});
	expect(short).toStrictEqual({ 0: 1 });
	expect(p[at(p, elapsed(clock, 13_000)).index].ms).toBe(60_000);
	expect(rows(p, elapsed(clock, 13_000), short)).toStrictEqual([1]);
});
