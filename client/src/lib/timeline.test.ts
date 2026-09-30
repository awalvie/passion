// @ts-nocheck: node runs this with `node --test`, and the client has no node types.
import assert from 'node:assert/strict';
import { test } from 'node:test';
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
} from './timeline.ts';

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
	assert.deepEqual(kinds(hang), [
		'prep5',
		'hang7', 'rest3', 'hang7', 'rest3', 'hang7', 'rest60',
		'hang7', 'rest3', 'hang7', 'rest3', 'hang7'
	]);
});

test('per side does all reps on the left, then the right, as two blocks', () => {
	const p = timeline({ ...hang, sets: 1, reps: 2, per_side: true, prep_seconds: 0 });
	assert.deepEqual(
		p.map((x) => `${x.kind}${x.side}${x.block}`),
		['hangLeft0', 'restLeft0', 'hangLeft0', 'restLeft0', 'hangRight1', 'restRight1', 'hangRight1']
	);
});

test('a phase with no time is left out', () => {
	assert.deepEqual(kinds({ ...hang, sets: 1, reps: 2, prep_seconds: null, rep_rest_seconds: 0 }), ['hang7', 'hang7']);
});

test('the clock leaves out pauses and adds skips', () => {
	const c: Clock = { startedAt: 1000, pausedAt: null, pausedMs: 500, skipMs: 2000 };
	assert.equal(elapsed(c, 11_000), 11_500);
	assert.equal(elapsed({ ...c, pausedAt: 6000 }, 99_000), 6500);
});

test('a pause stops the clock until it is resumed', () => {
	const paused = togglePause(newClock(0), 4000);
	assert.equal(elapsed(paused, 60_000), 4000);
	assert.equal(elapsed(togglePause(paused, 10_000), 11_000), 5000);
});

test('at finds the phase and the time left in it', () => {
	const p = timeline(hang);
	assert.deepEqual(at(p, 0), { index: 0, left: 5000 });
	assert.deepEqual(at(p, 6000), { index: 1, left: 6000 });
	assert.equal(at(p, 10 ** 9).index, p.length);
});

test('rows count each finished block, and the one in progress only when asked', () => {
	const p = timeline(hang);
	assert.deepEqual(rows(p, 5000 + 7000, {}), []);
	assert.deepEqual(rows(p, 5000 + 7000, {}, true), [1]);
	assert.deepEqual(rows(p, 5000 + 27_000, {}), [3]);
	assert.deepEqual(rows(p, 10 ** 9, {}), [3, 3]);
});

test('end set during prep changes nothing', () => {
	const p = timeline(hang);
	const c: Clock = { startedAt: 0, pausedAt: null, pausedMs: 0, skipMs: 0 };
	assert.deepEqual(endSet(p, c, 1000, {}), { clock: c, short: {} });
});

test('end set keeps the hangs done and moves to the rest after the set', () => {
	const p = timeline(hang);
	const c: Clock = { startedAt: 0, pausedAt: null, pausedMs: 0, skipMs: 0 };
	const { clock, short } = endSet(p, c, 5000 + 7000 + 1000, {});
	assert.deepEqual(short, { 0: 1 });
	assert.equal(p[at(p, elapsed(clock, 13_000)).index].ms, 60_000);
	assert.deepEqual(rows(p, elapsed(clock, 13_000), short), [1]);
});
