import { expect, test } from 'vitest';
import { climbWord, stepTime, type Climb, type LoggedSet, type RunStep } from './run';

test('climbWord names a flash, a send, many sends and no send, and counts goes on an ungraded climb', () => {
	expect(climbWord({ grade_system: 'font', goes: ['send'] })).toBe('Flash');
	expect(climbWord({ grade_system: 'font', goes: ['attempt', 'send'] })).toBe('Sent');
	expect(climbWord({ grade_system: 'font', goes: ['send', 'attempt', 'send'] })).toBe('2 sends');
	expect(climbWord({ grade_system: 'font', goes: ['attempt'] })).toBe('Not sent');
	expect(climbWord({ grade_system: null, goes: ['send', 'attempt'] })).toBe('2 goes');
});

const step = (kind: RunStep['kind'], status: RunStep['status'], elapsed: number | null) =>
	({ id: 's1', kind, status, elapsed_seconds: elapsed }) as RunStep;
const set = (seconds: number | null) => ({ step: 's1', number: 1, seconds }) as LoggedSet;
const climb = (seconds: number | null, position: number) => ({ step: 's1', position, seconds }) as Climb;
const none = { sets: [], climbs: [] };

test('stepTime shows a logged time exactly', () => {
	expect(stepTime({ sets: [set(1110)], climbs: [] }, step('open', 'done', 1500))).toBe('18m 30s');
	expect(stepTime({ sets: [], climbs: [climb(230, 0), climb(null, 1), climb(450, 2)] }, step('climbing', 'done', 2000))).toBe(
		'11m 20s'
	);
});

test('stepTime rounds the time a step ran to the minute, and keeps seconds under one', () => {
	expect(stepTime(none, step('open', 'done', 1110))).toBe('19m');
	expect(stepTime(none, step('climbing', 'done', 40))).toBe('40s');
	expect(stepTime({ sets: [], climbs: [climb(null, 0)] }, step('climbing', 'done', 600))).toBe('10m');
	expect(stepTime({ sets: [], climbs: [climb(0, 0)] }, step('climbing', 'done', 1500))).toBe('25m');
	expect(stepTime(none, step('open', 'done', 89))).toBe('1m');
	expect(stepTime(none, step('open', 'done', 90))).toBe('2m');
});

test('stepTime shows nothing for a skipped step, a set exercise or no time', () => {
	expect(stepTime(none, step('open', 'skipped', 600))).toBeNull();
	expect(stepTime(none, step('reps_and_sets', 'done', 600))).toBeNull();
	expect(stepTime(none, step('open', 'done', null))).toBeNull();
});
