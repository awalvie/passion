import { describe, expect, test } from 'vitest';
import { current, goalPeriod, left, pastPeriods, periodOf, periodTitle, spanTitle, through, ticked, type PeriodGoal } from './goals';

describe('periodOf', () => {
	test('a week runs Monday to Sunday', () => {
		expect(periodOf('week', '2026-11-01')).toEqual({ from: '2026-10-26', to: '2026-11-01' });
		expect(periodOf('week', '2026-10-26')).toEqual({ from: '2026-10-26', to: '2026-11-01' });
	});
	test('a week can cross into the next year', () => {
		expect(periodOf('week', '2026-12-31')).toEqual({ from: '2026-12-28', to: '2027-01-03' });
	});
	test('a month ends on its own last day', () => {
		expect(periodOf('month', '2026-02-10')).toEqual({ from: '2026-02-01', to: '2026-02-28' });
		expect(periodOf('month', '2028-02-10')).toEqual({ from: '2028-02-01', to: '2028-02-29' });
		expect(periodOf('month', '2026-04-30')).toEqual({ from: '2026-04-01', to: '2026-04-30' });
		expect(periodOf('month', '2026-12-31')).toEqual({ from: '2026-12-01', to: '2026-12-31' });
	});
	test('a year is the calendar year', () => {
		expect(periodOf('year', '2026-10-31')).toEqual({ from: '2026-01-01', to: '2026-12-31' });
	});
});

const goal = (o: Partial<PeriodGoal>): PeriodGoal => ({
	id: 'g',
	text: 'Train',
	period: 'week',
	starts: '2026-10-01',
	removed: null,
	span: null,
	ticks: [],
	...o
});

test('a goal is ticked for the period holding today', () => {
	const g = goal({ ticks: [{ period_start: '2026-10-19', text: 'Train' }] });
	expect(ticked(g, '2026-10-25')).toBe(true);
	expect(ticked(g, '2026-10-26')).toBe(false);
});

const trip = goal({
	id: 't',
	text: 'Send 7A',
	period: 'custom',
	starts: '2026-11-01',
	span: { id: 's', name: 'Font trip', starts: '2026-11-10', ends: '2026-11-20' }
});

test('a custom goal has one period', () => {
	const met = { ...trip, ticks: [{ period_start: '2026-11-10', text: 'Send 7A' }] };
	expect(goalPeriod(trip, '2026-12-25')).toEqual({ from: '2026-11-10', to: '2026-11-20' });
	expect(ticked(met, '2026-11-18')).toBe(true);
	expect(ticked(trip, '2026-11-18')).toBe(false);
});

test('a goal runs until it is removed or its days end', () => {
	const gone = goal({ id: 'r', removed: '2026-11-02' });
	expect(current([goal({}), gone, trip], '2026-11-20').map((g) => g.id)).toEqual(['g', 't']);
	expect(current([trip], '2026-11-21')).toEqual([]);
});

test('a span is headed by its name, or its dates', () => {
	const span = trip.span!;
	expect(spanTitle(span, '2026-11-01')).toEqual({ title: 'Font trip', note: 'Nov 10 – Nov 20 · starts in 9 days' });
	expect(spanTitle({ ...span, name: '' }, '2026-11-09')).toEqual({ title: 'Nov 10 – Nov 20', note: '11 days · starts tomorrow' });
	expect(spanTitle(span, '2026-11-10').note).toBe('Nov 10 – Nov 20 · ends in 10 days');
});

test('left counts the days to the end', () => {
	expect(left('2026-10-31', '2026-10-31')).toBe('ends today');
	expect(left('2026-11-01', '2026-10-31')).toBe('ends tomorrow');
	expect(left('2026-12-31', '2026-10-31')).toBe('ends in 61 days');
});

test('through counts today as begun, and stays within 0 and 1', () => {
	expect(through('2026-10-26', '2026-11-01', '2026-10-26')).toBeCloseTo(1 / 7);
	expect(through('2026-10-26', '2026-11-01', '2026-11-01')).toBe(1);
	expect(through('2026-11-10', '2026-11-20', '2026-11-01')).toBe(0);
	expect(through('2026-10-26', '2026-11-01', '2026-11-05')).toBe(1);
});

test('each period has a heading and a line', () => {
	expect(periodTitle('week', '2026-10-31')).toEqual({ title: 'This week', note: 'Oct 26 – Nov 1 · ends tomorrow' });
	expect(periodTitle('month', '2026-10-31')).toEqual({ title: 'October', note: 'This month · ends today' });
	expect(periodTitle('year', '2026-10-31')).toEqual({ title: '2026', note: 'This year · ends in 61 days' });
});

describe('pastPeriods', () => {
	test('a goal counts in each period that ended since it was set, the latest first', () => {
		const g = goal({ starts: '2026-10-21', ticks: [{ period_start: '2026-10-19', text: 'Train once' }] });
		const past = pastPeriods([g], '2026-11-04');
		expect(past.map((p) => p.key)).toEqual(['week-2026-10-26', 'week-2026-10-19']);
		expect(past[0]).toMatchObject({ title: 'Oct 26 – Nov 1', note: 'Week', goals: [{ text: 'Train', done: false }] });
		expect(past[1].goals).toEqual([{ id: 'g', text: 'Train once', done: true, removed: false }]);
	});
	test('goals of one period share it', () => {
		const a = goal({ id: 'a', period: 'month', starts: '2026-09-15' });
		const b = goal({ id: 'b', period: 'month', starts: '2026-10-03' });
		const past = pastPeriods([a, b], '2026-11-02');
		expect(past.map((p) => [p.title, p.goals.length])).toEqual([
			['October', 2],
			['September', 1]
		]);
	});
	test('a removed goal counts only in the periods that ended before it went', () => {
		const g = goal({ starts: '2026-10-19', removed: '2026-10-28' });
		const past = pastPeriods([g], '2026-11-10');
		expect(past.map((p) => p.key)).toEqual(['week-2026-10-19']);
		expect(past[0].goals[0].removed).toBe(true);
	});
	test('a goal removed in a period it was met in keeps that period', () => {
		const g = goal({ starts: '2026-10-19', removed: '2026-10-22', ticks: [{ period_start: '2026-10-19', text: 'Train' }] });
		expect(pastPeriods([g], '2026-11-10').map((p) => p.key)).toEqual(['week-2026-10-19']);
		expect(pastPeriods([{ ...g, ticks: [] }], '2026-11-10')).toEqual([]);
	});
	test('a removed span goal counts if its days ended first, or it was met', () => {
		expect(pastPeriods([{ ...trip, removed: '2026-11-25' }], '2026-12-01')).toHaveLength(1);
		expect(pastPeriods([{ ...trip, removed: '2026-11-15' }], '2026-12-01')).toEqual([]);
		const met = { ...trip, removed: '2026-11-15', ticks: [{ period_start: '2026-11-10', text: 'Send 7A' }] };
		expect(pastPeriods([met], '2026-12-01')[0].goals[0]).toMatchObject({ done: true, removed: true });
	});
	test('a year counts once it has ended', () => {
		const g = goal({ period: 'year', starts: '2025-03-01' });
		expect(pastPeriods([g], '2026-11-02').map((p) => [p.title, p.note])).toEqual([['2025', 'Year']]);
	});
	test('an earlier year is named with its month or week', () => {
		expect(pastPeriods([goal({ period: 'month', starts: '2025-12-10' })], '2026-01-05')[0].title).toBe('December 2025');
		const weeks = pastPeriods([goal({ starts: '2025-12-24' })], '2026-01-07');
		expect(weeks.map((p) => p.title)).toEqual(['Dec 29 – Jan 4', 'Dec 22 – Dec 28, 2025']);
	});
	test('days that ended are one period', () => {
		const other = { ...trip, id: 'u', text: 'Rest', span: { ...trip.span!, name: '' } };
		expect(pastPeriods([trip], '2026-11-20')).toEqual([]);
		expect(pastPeriods([trip], '2026-11-21')).toEqual([
			{
				key: 'span-s',
				title: 'Font trip',
				note: 'Nov 10 – Nov 20',
				from: '2026-11-10',
				to: '2026-11-20',
				goals: [{ id: 't', text: 'Send 7A', done: false, removed: false }]
			}
		]);
		expect(pastPeriods([other], '2026-11-21')[0]).toMatchObject({ title: 'Nov 10 – Nov 20', note: '11 days' });
	});
});
