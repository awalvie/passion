import { expect, test } from 'vitest';
import {
	climbWord,
	currentStop,
	doneWhen,
	isStale,
	lastDone,
	sectionMarks,
	stepTime,
	type Climb,
	type LoggedSet,
	type Run,
	type RunStep,
	type RunSummary
} from './run';

test('lastDone keeps each template’s newest finished run, and doneWhen says how long ago', () => {
	const summary = (template: string, local_date: string, finished = true) =>
		({ template, local_date, finished_at: finished ? `${local_date}T10:00:00Z` : null }) as RunSummary;
	const ran = lastDone([summary('a', '2026-10-24', false), summary('a', '2026-10-20'), summary('b', '2026-10-25'), summary('a', '2026-10-01')]);
	expect(ran.get('a')).toBe('2026-10-20');
	expect(doneWhen(ran.get('a'), '2026-10-25')).toBe('Last done 5 days ago');
	expect(doneWhen(ran.get('b'), '2026-10-25')).toBe('Done today');
	expect(doneWhen('2026-10-24', '2026-10-25')).toBe('Last done yesterday');
	expect(doneWhen(undefined, '2026-10-25')).toBe('Not done yet');
});

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

test('sectionMarks marks a section with every step skipped as skipped, not done', () => {
	const section = (...steps: RunStep[]) => ({ items: steps.map((s) => ({ step: s })) });
	const run = {
		sections: [
			section(step('open', 'skipped', null)),
			section(step('open', 'skipped', null), step('open', 'done', 60)),
			section(step('open', null, null)),
			section(step('open', null, null))
		]
	} as unknown as Pick<Run, 'sections'>;
	expect(sectionMarks(run, 3)).toEqual(['skipped', 'done', 'todo', 'now']);
});

test('a skipped choice finishes its section and is no stop', () => {
	const choice = (skipped: boolean) => ({ choice: { id: 'c1', pick: 1, skipped } });
	const run = {
		sections: [
			{ items: [choice(true)] },
			{ items: [{ step: step('open', 'done', 60) }, choice(true)] },
			{ items: [choice(false)] }
		]
	} as unknown as Pick<Run, 'sections'>;
	expect(sectionMarks(run, -1)).toEqual(['skipped', 'done', 'todo']);
	expect(currentStop(run)).toBe(run.sections[2].items[0]);
});

test('isStale keeps an unfinished run open until 04:00 after its day', () => {
	const open = { finished_at: null, local_date: '2026-10-24', timezone: 'Asia/Kolkata' };
	const at = (iso: string) => new Date(iso);
	expect(isStale(open, at('2026-10-24T23:30:00+05:30'))).toBe(false);
	expect(isStale(open, at('2026-10-25T03:59:00+05:30'))).toBe(false);
	expect(isStale(open, at('2026-10-25T04:00:00+05:30'))).toBe(true);
	expect(isStale({ ...open, finished_at: '2026-10-24T20:00:00Z' }, at('2026-10-27T12:00:00+05:30'))).toBe(false);
});

// The cutoff is 04:00 wall time in the run's own zone, wherever the phone is now.
test.each([
	['Pacific/Auckland', '2026-10-24', '2026-10-24T14:59:59Z', false],
	['Pacific/Auckland', '2026-10-24', '2026-10-24T15:00:00Z', true],
	['America/Los_Angeles', '2026-10-24', '2026-10-25T10:59:59Z', false],
	['America/Los_Angeles', '2026-10-24', '2026-10-25T11:00:00Z', true],
	['UTC', '2026-12-31', '2027-01-01T03:59:59Z', false],
	['UTC', '2026-12-31', '2027-01-01T04:00:00Z', true],
	['UTC', '2028-02-29', '2028-03-01T03:59:59Z', false],
	['UTC', '2028-02-29', '2028-03-01T04:00:00Z', true]
])('isStale in %s for a run of %s is judged at %s: %s', (timezone, local_date, now, want) => {
	expect(isStale({ finished_at: null, local_date, timezone }, new Date(now))).toBe(want);
});

test('isStale is false on the day itself, and for a day still to come', () => {
	const at = (local_date: string) => isStale({ finished_at: null, local_date, timezone: 'UTC' }, new Date('2026-10-24T12:00:00Z'));
	expect(at('2026-10-24')).toBe(false);
	expect(at('2026-10-25')).toBe(false);
	expect(at('2026-10-23')).toBe(true);
});

// A write-up of a day already gone starts with that day on it, so it is stale
// from the first second, and the finish page asks for its length.
test('isStale holds for a run opened now on a past day', () => {
	const now = new Date('2026-10-24T12:00:00Z');
	expect(isStale({ finished_at: null, local_date: '2026-03-07', timezone: 'Europe/Oslo' }, now)).toBe(true);
});

// 04:00 on the wall, as the owner reads it, even on the night the clocks move.
// Europe/Berlin fell back at 01:00Z on 2026-10-25.
test('isStale holds until 04:00 on the wall on the night the clocks go back', () => {
	const open = { finished_at: null, local_date: '2026-10-24', timezone: 'Europe/Berlin' };
	expect(isStale(open, new Date('2026-10-25T02:30:00Z')), '03:30 on the wall').toBe(false);
	expect(isStale(open, new Date('2026-10-25T03:00:00Z')), '04:00 on the wall').toBe(true);
});

// Europe/Berlin sprang forward at 01:00Z on 2026-03-29.
test('isStale turns at 04:00 on the wall on the night the clocks go forward', () => {
	const open = { finished_at: null, local_date: '2026-03-28', timezone: 'Europe/Berlin' };
	expect(isStale(open, new Date('2026-03-29T01:59:00Z')), '03:59 on the wall').toBe(false);
	expect(isStale(open, new Date('2026-03-29T02:00:00Z')), '04:00 on the wall').toBe(true);
});
