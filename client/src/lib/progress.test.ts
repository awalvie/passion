import { expect, test } from 'vitest';
import type { Scale } from './grades';
import { addDays } from './dates';
import {
	bestSet,
	buckets,
	climbMarks,
	figure,
	averageSend,
	counts,
	filterName,
	flashRate,
	goesPerSend,
	grades,
	hours,
	mixOf,
	filterQuery,
	headline,
	measure,
	monthsBack,
	passes,
	perBucket,
	readFilter,
	readSpan,
	sendRate,
	setMarks,
	setValue,
	spread,
	tenWeeks,
	topSend,
	type DatedClimb
} from './progress';
import type { Climb, HistorySession, RunSummary, SetFields } from './run';

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

test('a filter widens within a group and narrows across groups', () => {
	const c = (discipline: string, setting: string, board: string | null, rope_style: string | null) =>
		({ discipline, setting, board, rope_style }) as Climb;
	const moon = c('boulder', 'indoor', 'moon', null);
	const gym = c('boulder', 'indoor', null, null);
	const crag = c('boulder', 'outdoor', null, null);
	const lead = c('sport', 'indoor', null, 'lead');
	const belay = c('sport', 'indoor', null, 'auto_belay_rope');
	const trad = c('trad', 'outdoor', null, 'lead');
	const all = [moon, gym, crag, lead, belay, trad];
	const keep = (q: string) => all.filter((x) => passes(x, readFilter(new URLSearchParams(q))));
	expect(keep('')).toEqual(all);
	expect(keep('discipline=sport,trad')).toEqual([lead, belay, trad]);
	expect(keep('discipline=sport,trad&setting=indoor')).toEqual([lead, belay]);
	expect(keep('discipline=trad')).toEqual([trad]);
	expect(keep('board=moon')).toEqual([moon]);
	// A board closes when only outdoor is picked.
	expect(keep('setting=outdoor&board=moon')).toEqual([crag, trad]);
	expect(keep('discipline=nope&board=none')).toEqual(all);
	// A board narrows boulders, not the routes picked beside them.
	expect(keep('discipline=boulder,sport&board=moon')).toEqual([moon, lead, belay]);
	expect(keep('discipline=constructor&board=toString')).toEqual(all);
});

test('a filter goes to the URL and back', () => {
	const f = readFilter(new URLSearchParams('discipline=boulder&board=moon,kilter&rope=lead'));
	expect(filterQuery(f)).toBe('&discipline=boulder&board=moon,kilter');
	expect(filterQuery(readFilter(new URLSearchParams('discipline=sport&board=moon')))).toBe('&discipline=sport');
});

test('filterName says what the filter shows', () => {
	const name = (q: string) => filterName(readFilter(new URLSearchParams(q)));
	expect(name('discipline=boulder,sport&board=moon')).toBe('Moon boulders and sport routes');
	expect(name('discipline=sport,sport')).toBe('Sport routes');
	expect(name('')).toBe('All climbs');
	expect(name('discipline=sport,trad&setting=indoor')).toBe('Indoor routes');
	expect(name('discipline=sport,trad&setting=indoor,outdoor')).toBe('Routes');
	expect(name('discipline=sport')).toBe('Sport routes');
	expect(name('discipline=trad&setting=outdoor')).toBe('Outdoor trad routes');
	expect(name('discipline=boulder,sport')).toBe('Boulders and sport routes');
	expect(name('discipline=boulder,sport,trad&setting=indoor')).toBe('Indoor boulders and routes');
	expect(name('board=tension,spray')).toBe('Tension, Spray boulders');
	expect(name('discipline=boulder&setting=outdoor')).toBe('Outdoor boulders');
	expect(name('setting=indoor')).toBe('Indoor climbs');
});

test('mixOf says where a climb was', () => {
	const c = (discipline: string, setting: string, board: string | null) => mixOf({ discipline, setting, board } as Climb);
	expect([c('boulder', 'indoor', 'moon'), c('boulder', 'indoor', 'none'), c('boulder', 'outdoor', null), c('trad', 'outdoor', null), c('sport', 'indoor', null)]).toEqual([
		'Moon board',
		'Gym boulders',
		'Outdoor boulders',
		'Outdoor routes',
		'Indoor routes'
	]);
});

test('the rates count graded climbs only', () => {
	const go = (grade: string | null, goes: string[]) => ({ grade, grade_system: grade && 'font', goes, sent: goes.includes('send') }) as Climb;
	const cs = [go('6a', ['send']), go('6a', ['attempt', 'attempt', 'send']), go('6a', ['attempt']), go(null, ['send'])];
	expect(sendRate(cs)).toBe(67);
	expect(flashRate(cs)).toBe(33);
	expect(goesPerSend(cs.slice(0, 3))).toBe(2);
	expect(sendRate([go(null, ['send'])])).toBeNull();
	expect(goesPerSend([])).toBeNull();
});

test('averageSend is the mean grade sent, to the nearest grade', () => {
	expect(averageSend([climb('6a', true), climb('6c', true), climb('7a', false)], scale)).toBe(1);
	expect(averageSend([climb('7a', false)], scale)).toBeNull();
});

test('spread counts each grade from the easiest climbed to the hardest', () => {
	expect(spread([climb('7a', false), climb('6b', true), climb('6b', false), climb('5c', true)], scale)).toEqual([
		{ grade: '6b', sent: 1, not: 1 },
		{ grade: '6c', sent: 0, not: 0 },
		{ grade: '7a', sent: 0, not: 1 }
	]);
	expect(spread([climb('5c', true)], scale)).toEqual([]);
});

test('perBucket gives each bucket its own figure and null where it has no climbs', () => {
	const bs = [
		{ from: '2026-10-05', to: '2026-10-11', tick: '' },
		{ from: '2026-10-12', to: '2026-10-18', tick: '' },
		{ from: '2026-10-19', to: '2026-10-25', tick: '' }
	];
	const at = (local_date: string, grade: string, sent: boolean) => ({ ...climb(grade, sent), local_date, run: 'r' }) as DatedClimb;
	const cs = [at('2026-10-05', '6a', true), at('2026-10-11', '6a', false), at('2026-10-19', '6a', true)];
	expect(perBucket(cs, bs, sendRate)).toEqual([50, null, 100]);
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

// Saturday.
const today = '2026-10-24';
const cycles = [{ id: 'c', name: 'Base', starts: '2026-10-01', ends: '2026-10-21' }];

test('ten weeks run Monday to Sunday, ticked where a month starts', () => {
	const { buckets: bs, now, per } = buckets(tenWeeks(today), today);
	expect(bs.map((b) => b.from)).toEqual([
		'2026-08-17', '2026-08-24', '2026-08-31', '2026-09-07', '2026-09-14',
		'2026-09-21', '2026-09-28', '2026-10-05', '2026-10-12', '2026-10-19'
	]);
	expect(bs.at(-1)!.to).toBe('2026-10-25');
	expect(bs.map((b) => b.tick)).toEqual(['', '', 'Sep', '', '', '', 'Oct', '', '', '']);
	expect(now).toBe(9);
	expect(per).toBe('week');
});

test("a cycle's weeks count from its first day", () => {
	const past = buckets(readSpan('cycle:c', today, cycles), today);
	expect(past.buckets).toEqual([
		{ from: '2026-10-01', to: '2026-10-07', tick: 'W1' },
		{ from: '2026-10-08', to: '2026-10-14', tick: 'W2' },
		{ from: '2026-10-15', to: '2026-10-21', tick: 'W3' }
	]);
	expect(past.now).toBe(-1);
	expect(buckets(readSpan('cycle:c', '2026-10-10', cycles), '2026-10-10').now).toBe(1);
});

test('a custom span starts its first bucket on its first day', () => {
	const { buckets: bs, now } = buckets(readSpan('2026-09-25:2026-10-24', today, cycles), today);
	expect(bs[0]).toEqual({ from: '2026-09-25', to: '2026-09-27', tick: '' });
	expect(bs.at(-1)!.from).toBe('2026-10-19');
	expect(now).toBe(bs.length - 1);
});

test('a custom span splits into months past 190 days', () => {
	const weeks = buckets(readSpan(`${addDays(today, -190)}:${today}`, today, cycles), today);
	expect(weeks.per).toBe('week');
	const from = addDays(today, -191);
	const months = buckets(readSpan(`${from}:${today}`, today, cycles), today);
	expect(months.per).toBe('month');
	expect(months.buckets[0].from).toBe(from);
	expect(months.buckets[0].tick).toBe('Apr');
	expect(months.buckets.at(-1)).toEqual({ from: '2026-10-01', to: '2026-10-31', tick: 'Oct' });
});

test('readSpan falls back to ten weeks and stops a span at today', () => {
	expect(readSpan(null, today, cycles).key).toBe('10w');
	expect(readSpan('6m', today, cycles).key).toBe('10w');
	expect(readSpan('cycle:nope', today, cycles).key).toBe('10w');
	expect(readSpan('cycle:c', '2026-09-01', cycles).key).toBe('10w');
	expect(readSpan('2026-10-20:2026-10-01', today, cycles).key).toBe('10w');
	expect(readSpan('2026-11-01:2026-11-30', today, cycles).key).toBe('10w');
	expect(readSpan('2026-10-01:2026-12-31', today, cycles).to).toBe(today);
	expect(readSpan('2026-02-30:2026-03-01', today, cycles).key).toBe('10w');
});

test('monthsBack ends today and starts the day after the same date', () => {
	expect(monthsBack(1, today)).toBe('2026-09-25:2026-10-24');
	expect(monthsBack(12, today)).toBe('2025-10-25:2026-10-24');
	expect(monthsBack(1, '2026-03-31')).toBe('2026-03-01:2026-03-31');
	expect(monthsBack(6, '2026-08-31')).toBe('2026-03-01:2026-08-31');
	expect(monthsBack(12, '2028-02-29')).toBe('2027-03-01:2028-02-29');
});

test('a cycle longer than 16 weeks names every fourth week', () => {
	const long = [{ id: 'l', name: 'Year', starts: '2026-01-05', ends: '2026-12-27' }];
	const ticks = buckets(readSpan('cycle:l', today, long), today).buckets.map((b) => b.tick);
	expect(ticks.slice(0, 6)).toEqual(['W1', '', '', '', 'W5', '']);
});

const dated = (local_date: string, over: Partial<Climb> = {}) =>
	({ ...climb('6a', true), discipline: 'boulder', setting: 'indoor', board: null, local_date, run: 'r', ...over }) as DatedClimb;

test('hours puts a session in the week of its day, and counts an unknown length as nothing', () => {
	const run = (local_date: string, elapsed_seconds: number | null) => ({ local_date, elapsed_seconds }) as RunSummary;
	const { buckets: bs } = buckets(tenWeeks(today), today);
	const got = hours(
		[
			run('2026-10-19', 3600),
			run('2026-10-25', 1800),
			// Sunday ends a week and Monday starts the next.
			run('2026-10-18', 7200),
			run('2026-10-12', null),
			run('2026-08-16', 3600),
			run('2026-10-26', 3600)
		],
		bs
	);
	expect(got).toEqual([0, 0, 0, 0, 0, 0, 0, 0, 2, 1.5]);
});

test('counts and grades keep to the days of each bucket, and boulders apart from routes', () => {
	const bs = [
		{ from: '2026-10-05', to: '2026-10-11', tick: '' },
		{ from: '2026-10-12', to: '2026-10-18', tick: '' }
	];
	const cs = [
		dated('2026-10-11', { grade: '6b' }),
		dated('2026-10-11', { discipline: 'sport', grade: '7a', sent: false }),
		dated('2026-10-12', { discipline: 'trad', grade: '6c' }),
		dated('2026-10-18', { grade: '6a' }),
		dated('2026-10-18', { grade: '6c' }),
		dated('2026-10-19', { grade: '7a' })
	];
	expect(counts(cs, bs)).toEqual([
		{ boulders: 1, routes: 1 },
		{ boulders: 2, routes: 1 }
	]);
	// The hardest send, in the scale: 7a was not sent, and 6c on the 19th is outside.
	expect(grades(cs, bs, scale)).toEqual([1, 2]);
	expect(grades([], bs, scale)).toEqual([null, null]);
	expect(grades([dated('2026-10-11', { grade_system: 'v' })], bs, scale)).toEqual([null, null]);
});

test('a span of one day is one bucket, cut to that day at its start', () => {
	const sunday = '2026-10-18';
	const span = readSpan(`${sunday}:${sunday}`, today, cycles);
	expect(span).toMatchObject({ from: sunday, to: sunday });
	const { buckets: bs, now } = buckets(span, today);
	expect(bs).toEqual([{ from: sunday, to: sunday, tick: '' }]);
	expect(now).toBe(-1);
	const onToday = buckets(readSpan(`${today}:${today}`, today, cycles), today);
	expect(onToday.buckets).toEqual([{ from: today, to: '2026-10-25', tick: '' }]);
	expect(onToday.now).toBe(0);
});

test('readSpan refuses a key it cannot read, and a key with room to spare', () => {
	for (const key of ['', ' 2026-10-01:2026-10-20', '2026-10-01:2026-10-20 ', '2026-10-1:2026-10-20', '2026-10-01-2026-10-20', 'cycle:', 'cycle', '10W', '2026-13-01:2026-13-02']) {
		expect(readSpan(key, today, cycles).key, JSON.stringify(key)).toBe('10w');
	}
	expect(readSpan('2026-10-24:2026-10-24', today, cycles).key).toBe('2026-10-24:2026-10-24');
});

test('a cycle that began today shows; one still to come does not; a running one ends on its last day', () => {
	const c = [
		{ id: 'now', name: 'Now', starts: today, ends: '2026-11-20' },
		{ id: 'next', name: 'Next', starts: '2026-10-25', ends: '2026-11-20' }
	];
	const now = readSpan('cycle:now', today, c);
	expect(now).toMatchObject({ from: today, to: '2026-11-20', cycle: { id: 'now', name: 'Now' } });
	expect(readSpan('cycle:next', today, c).key).toBe('10w');
	const { buckets: bs, now: at } = buckets(now, today);
	expect(bs).toHaveLength(4);
	expect(bs.at(-1)).toEqual({ from: '2026-11-14', to: '2026-11-20', tick: 'W4' });
	expect(at).toBe(0);
});

test('a span of ten months is cut into months, the first starting on its first day', () => {
	const { buckets: bs, now, per } = buckets(readSpan('2026-01-15:2026-10-24', today, cycles), today);
	expect(per).toBe('month');
	expect(bs).toHaveLength(10);
	expect(bs[0]).toEqual({ from: '2026-01-15', to: '2026-01-31', tick: 'Jan' });
	expect(bs[1]).toEqual({ from: '2026-02-01', to: '2026-02-28', tick: 'Feb' });
	expect(now).toBe(9);
});

test('ten weeks over the new year name the month the new year falls in', () => {
	const ny = '2027-01-10';
	const ticks = buckets(tenWeeks(ny), ny).buckets.map((b) => b.tick);
	expect(ticks).toEqual(['', '', '', '', 'Dec', '', '', '', 'Jan', '']);
});

test('a board picked alone shows no routes, and a route type picked with it shows them whole', () => {
	const c = (discipline: string, board: string | null) => ({ discipline, setting: 'indoor', board }) as Climb;
	const f = (q: string) => readFilter(new URLSearchParams(q));
	expect(passes(c('boulder', 'custom'), f('board=custom'))).toBe(true);
	expect(passes(c('boulder', 'none'), f('board=custom'))).toBe(false);
	expect(passes(c('boulder', null), f('board=custom'))).toBe(false);
	expect(passes(c('sport', null), f('board=custom'))).toBe(false);
	expect(passes(c('sport', null), f('discipline=boulder,sport&board=custom'))).toBe(true);
	expect(passes(c('trad', null), f('discipline=boulder,sport&board=custom'))).toBe(false);
	expect(passes(c('boulder', null), f(''))).toBe(true);
});

// The filter shows indoor routes only, so the title must not read as every route.
test('filterName keeps Indoor in the name when a board is picked beside route types', () => {
	const name = (q: string) => filterName(readFilter(new URLSearchParams(q)));
	expect(name('discipline=boulder,sport&board=moon&setting=indoor')).toMatch(/indoor/i);
	expect(name('discipline=boulder,sport&setting=indoor')).toBe('Indoor boulders and sport routes');
});
