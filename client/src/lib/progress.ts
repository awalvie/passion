import { addDays, daysBetween, formatDate, mondayOf } from './dates';
import type { Scale } from './grades';
import { boards, disciplines, settings, type Climb, type HistorySession, type RunSummary, type SetFields } from './run';

export type DatedClimb = Climb & { run: string; local_date: string };

// A span of days a chart covers: the last ten weeks, a cycle, or any two days.
export type Span = { key: string; from: string; to: string; cycle: { id: string; name: string } | null };

// tenWeeks ends with the week holding today.
export function tenWeeks(today: string): Span {
	return { key: '10w', from: addDays(mondayOf(today), -63), to: today, cycle: null };
}

// readSpan reads a span from a URL's range: 10w, cycle:<id> or from:to. Any
// other value is the last ten weeks, and a custom span ends today at the latest.
export function readSpan(key: string | null, today: string, cycles: { id: string; name: string; starts: string; ends: string }[]): Span {
	const c = key?.startsWith('cycle:') ? cycles.find((c) => c.id === key.slice(6)) : undefined;
	if (c && c.starts <= today) return { key: key!, from: c.starts, to: c.ends, cycle: { id: c.id, name: c.name } };
	const m = key?.match(/^(\d{4}-\d{2}-\d{2}):(\d{4}-\d{2}-\d{2})$/);
	if (m && real(m[1]) && real(m[2]) && m[1] <= m[2] && m[1] <= today) return { key: key!, from: m[1], to: m[2] < today ? m[2] : today, cycle: null };
	return tenWeeks(today);
}

// A URL can name a day no calendar has, such as Feb 30.
const real = (day: string) => {
	const t = Date.parse(`${day}T00:00:00Z`);
	return !Number.isNaN(t) && new Date(t).toISOString().slice(0, 10) === day;
};

// monthsBack is the custom span of the last n months, ending today. A day the
// earlier month lacks becomes its last, so a month back from Mar 31 starts
// Mar 1.
export function monthsBack(n: number, today: string): string {
	const [y, m, d] = today.split('-').map(Number);
	const month = new Date(Date.UTC(y, m - 1 - n, 1));
	const last = new Date(Date.UTC(month.getUTCFullYear(), month.getUTCMonth() + 1, 0)).getUTCDate();
	month.setUTCDate(Math.min(d, last));
	return `${addDays(month.toISOString().slice(0, 10), 1)}:${today}`;
}

// tick is the month's name on a chart's time axis, set only on the bucket a
// month starts in, or a cycle's week number.
export type Bucket = { from: string; to: string; tick: string };

const nextMonth = (first: string) => {
	const d = new Date(`${first}T00:00:00Z`);
	d.setUTCMonth(d.getUTCMonth() + 1);
	return d.toISOString().slice(0, 10);
};

// buckets splits a span into weeks, or into months past 190 days, oldest
// first. A cycle's weeks count from its first day; other weeks run Monday to
// Sunday. The first bucket starts no earlier than the span. now is the
// bucket holding today, or -1 for a span that is over.
export function buckets(s: Span, today: string): { buckets: Bucket[]; now: number; per: 'week' | 'month' } {
	const out: Bucket[] = [];
	const per = !s.cycle && daysBetween(s.from, s.to) > 190 ? 'month' : 'week';
	if (s.cycle) {
		// A long cycle names every fourth week, so the ticks stay readable.
		const every = daysBetween(s.from, s.to) > 16 * 7 ? 4 : 1;
		for (let from = s.from, w = 1; from <= s.to; from = addDays(from, 7), w++) {
			const to = addDays(from, 6);
			out.push({ from, to: to < s.to ? to : s.to, tick: (w - 1) % every ? '' : `W${w}` });
		}
	} else if (per === 'month') {
		for (let first = `${s.from.slice(0, 7)}-01`; first <= s.to; first = nextMonth(first)) {
			out.push({ from: first < s.from ? s.from : first, to: addDays(nextMonth(first), -1), tick: formatDate(first, { month: 'short' }) });
		}
	} else {
		for (let monday = mondayOf(s.from); monday <= s.to; monday = addDays(monday, 7)) {
			const to = addDays(monday, 6);
			const tick = monday.slice(0, 7) === to.slice(0, 7) && !monday.endsWith('-01') ? '' : formatDate(to, { month: 'short' });
			out.push({ from: monday < s.from ? s.from : monday, to, tick });
		}
	}
	return { buckets: out, now: out.findIndex((b) => b.from <= today && today <= b.to), per };
}

const within = (b: Bucket, date: string) => b.from <= date && date <= b.to;

// hours is the logged training time in each bucket.
export function hours(runs: RunSummary[], bs: Bucket[]): number[] {
	return bs.map((b) => runs.reduce((h, r) => h + (within(b, r.local_date) ? (r.elapsed_seconds ?? 0) / 3600 : 0), 0));
}

export const isBoulder = (c: Pick<Climb, 'discipline'>) => c.discipline === 'boulder';

// A filter on climbs, grouped as a climb's setup is. A climb shows when it
// matches a pick in every group that has one.
export const filterGroups = {
	discipline: { label: 'Discipline', options: disciplines },
	setting: { label: 'Indoor or outdoor', options: settings },
	board: { label: 'Board', options: { kilter: boards.kilter, moon: boards.moon, tension: boards.tension, spray: boards.spray, custom: boards.custom } }
};
export type FilterGroup = keyof typeof filterGroups;
export type ClimbFilter = Record<FilterGroup, string[]>;
const groups = Object.keys(filterGroups) as FilterGroup[];

// A board is only for an indoor boulder, so boards close when only routes or
// only outdoor is picked.
export const boardsOpen = (f: ClimbFilter) =>
	(!f.discipline.length || f.discipline.includes('boulder')) && !(f.setting.length === 1 && f.setting[0] === 'outdoor');

export function readFilter(params: URLSearchParams): ClimbFilter {
	const f = {} as ClimbFilter;
	for (const g of groups) f[g] = [...new Set((params.get(g) ?? '').split(','))].filter((v) => Object.hasOwn(filterGroups[g].options, v));
	if (!boardsOpen(f)) f.board = [];
	return f;
}

// filterName says what the filter shows as the owner would, such as Indoor
// routes or Tension, Spray boulders. A board means an indoor boulder, and
// sport with trad is every route.
export function filterName(f: ClimbFilter): string {
	const names = f.board.map((b) => filterGroups.board.options[b as keyof typeof filterGroups.board.options]);
	const routes = f.discipline.filter((d) => d !== 'boulder');
	const kinds = [
		names.length ? `${names.join(', ')} boulders` : f.discipline.includes('boulder') ? 'boulders' : '',
		routes.length === 2 ? 'routes' : routes.length ? `${routes[0]} routes` : ''
	].filter(Boolean);
	const where = !names.length && f.setting.length === 1 ? settings[f.setting[0] as keyof typeof settings] : '';
	const words = [where, kinds.join(' and ') || 'climbs'].filter(Boolean).join(' ');
	return words === 'climbs' ? 'All climbs' : words[0].toUpperCase() + words.slice(1);
}

// filterQuery is the filter as URL parameters, each led by &.
export const filterQuery = (f: ClimbFilter) =>
	groups
		.filter((g) => f[g].length)
		.map((g) => `&${g}=${f[g].join(',')}`)
		.join('');

// A board narrows only boulders: a route still shows when its discipline was
// picked.
export const passes = (c: Pick<Climb, FilterGroup>, f: ClimbFilter) =>
	(!f.discipline.length || f.discipline.includes(c.discipline)) &&
	(!f.setting.length || f.setting.includes(c.setting)) &&
	(!f.board.length || (isBoulder(c) ? f.board.includes(c.board ?? '') : f.discipline.includes(c.discipline)));

// mixOf is where a climb was, for the mix of climbing: on which board, in
// the gym or outside.
export function mixOf(c: Pick<Climb, FilterGroup>): string {
	if (!isBoulder(c)) return c.setting === 'outdoor' ? 'Outdoor routes' : 'Indoor routes';
	if (c.setting === 'outdoor') return 'Outdoor boulders';
	return c.board && c.board !== 'none' ? `${boards[c.board as keyof typeof boards]} board` : 'Gym boulders';
}

// The rates below count graded climbs only: an ungraded climb is never a send.
const graded = <T extends Pick<Climb, 'grade_system'>>(climbs: T[]) => climbs.filter((c) => c.grade_system);
const percent = (part: number, whole: number) => (whole ? Math.round((part / whole) * 100) : null);

// sendRate is the share of graded climbs sent, as a whole percent.
export const sendRate = (climbs: Pick<Climb, 'sent' | 'grade_system'>[]) => percent(graded(climbs).filter((c) => c.sent).length, graded(climbs).length);

// flashRate is the share of graded climbs sent on the first go.
export const flashRate = (climbs: Pick<Climb, 'goes' | 'grade_system'>[]) => percent(graded(climbs).filter((c) => c.goes[0] === 'send').length, graded(climbs).length);

// goesPerSend is how many goes a send took on average, to one decimal.
export function goesPerSend(climbs: Pick<Climb, 'goes' | 'sent'>[]): number | null {
	const sent = climbs.filter((c) => c.sent);
	return sent.length ? Math.round((sent.reduce((n, c) => n + c.goes.indexOf('send') + 1, 0) / sent.length) * 10) / 10 : null;
}

// perBucket is a figure for each bucket's climbs, null for a bucket without any.
export function perBucket<T extends DatedClimb>(climbs: T[], bs: Bucket[], figure: (cs: T[]) => number | null): (number | null)[] {
	return bs.map((b) => {
		const mine = climbs.filter((c) => within(b, c.local_date));
		return mine.length ? figure(mine) : null;
	});
}

// counts is the number of climbs in each bucket, boulders and routes apart.
export function counts(climbs: DatedClimb[], bs: Bucket[]): { boulders: number; routes: number }[] {
	return bs.map((b) => {
		const mine = climbs.filter((c) => within(b, c.local_date));
		const boulders = mine.filter(isBoulder).length;
		return { boulders, routes: mine.length - boulders };
	});
}

// gradeScale is the scale most of these climbs were graded in.
export function gradeScale(climbs: Climb[], scales: Scale[]): Scale | null {
	const n = new Map<string, number>();
	for (const c of climbs) if (c.grade_system) n.set(c.grade_system, (n.get(c.grade_system) ?? 0) + 1);
	const top = [...n].sort((a, b) => b[1] - a[1])[0]?.[0];
	return scales.find((s) => s.system === top) ?? null;
}

// hardest is the hardest send, as a position in the scale, or null when none
// is graded in it.
function hardest(climbs: Climb[], scale: Scale): number | null {
	const sent = climbs
		.filter((c) => c.sent && c.grade_system === scale.system)
		.map((c) => scale.grades.indexOf(c.grade!))
		.filter((i) => i >= 0);
	return sent.length ? Math.max(...sent) : null;
}

// grades is the hardest send in each bucket.
export function grades(climbs: DatedClimb[], bs: Bucket[], scale: Scale): (number | null)[] {
	return bs.map((b) => hardest(climbs.filter((c) => within(b, c.local_date)), scale));
}

// An exercise's history charts one measure: the added weight where any set
// carries some, then the time under load where the sets are timed, then reps.
export type Measure = 'kg' | 's' | 'reps';

const timed = (x: SetFields) => !!(x.seconds || x.rep_seconds?.length);

// A set holds something when it was done: a run can log a row left empty, and
// a timed set whose every hang was skipped has 0 reps.
const done = (x: SetFields) => !!(x.weight_kg || x.reps || (x.reps === null && timed(x)));

export function measure(sessions: HistorySession[]): Measure {
	const sets = sessions.flatMap((s) => s.sets).filter(done);
	if (sets.some((x) => x.weight_kg)) return 'kg';
	if (sets.some(timed) && sets.every(timed)) return 's';
	return 'reps';
}

export function setValue(x: SetFields, m: Measure): number {
	if (m === 'kg') return x.weight_kg ?? 0;
	if (m === 's') return x.rep_seconds?.length ? x.rep_seconds.reduce((a, b) => a + b, 0) : (x.reps ?? 1) * (x.seconds ?? 0);
	return x.reps ?? 0;
}

// A tie on the measure goes to the set with more reps or seconds.
const score = (x: SetFields, m: Measure) => setValue(x, m) * 1000 + (x.reps ?? x.seconds ?? 0);

export function bestSet<T extends SetFields>(sets: T[], m: Measure): T | null {
	return sets.filter(done).reduce<T | null>((b, x) => (!b || score(x, m) > score(b, m) ? x : b), null);
}

export type Mark = { i: number; v: number; best?: boolean; hollow?: boolean };

// setMarks are the sets of the session in slot i, the best leading, each
// other value once.
export function setMarks(sets: SetFields[], i: number, m: Measure): Mark[] {
	const b = bestSet(sets, m);
	if (!b) return [];
	const top = setValue(b, m);
	const rest = new Set(
		sets
			.filter(done)
			.map((x) => setValue(x, m))
			.filter((v) => v !== top)
	);
	return [{ i, v: top, best: true }, ...[...rest].map((v) => ({ i, v }))];
}

const rank = (c: Climb, scale: Scale) => (c.grade_system === scale.system ? scale.grades.indexOf(c.grade!) : -1);

export function topSend(climbs: Climb[], scale: Scale): Climb | null {
	return climbs.filter((c) => c.sent && rank(c, scale) >= 0).sort((a, b) => rank(b, scale) - rank(a, scale))[0] ?? null;
}

// climbMarks are the graded climbs of the session in slot i. The hardest
// send is the best; a climb not sent is hollow.
export function climbMarks(climbs: Climb[], i: number, scale: Scale): Mark[] {
	const lead = topSend(climbs, scale);
	return climbs.filter((c) => rank(c, scale) >= 0).map((c) => ({ i, v: rank(c, scale), best: c === lead, hollow: !c.sent }));
}

export const clock = (s: number) => `${Math.floor(Math.abs(s) / 60)}:${String(Math.abs(s) % 60).padStart(2, '0')}`;

// figure writes a set as its number and its unit. Short leaves out a weighted
// ladder's seconds.
export function figure(x: SetFields, m: Measure, short = false): [string, string] {
	const w = x.weight_kg ?? 0;
	const ladder = x.rep_seconds?.length ? x.rep_seconds.join('/') : null;
	if (m === 'kg' && w) {
		if (ladder) return short ? [`${w}`, 'kg'] : [`${w}`, `kg · ${ladder} s`];
		if (x.reps !== null) return [`${x.reps} × ${w}`, 'kg'];
		return x.seconds ? [`${w}`, `kg · ${x.seconds} s`] : [`${w}`, 'kg'];
	}
	if (ladder) return [ladder, 's'];
	if (x.reps !== null) return x.seconds ? [`${x.reps} × ${x.seconds}`, 's'] : [`${x.reps}`, 'reps'];
	return (x.seconds ?? 0) >= 60 ? [clock(x.seconds ?? 0), 'min'] : [`${x.seconds ?? 0}`, 's'];
}

// headline is the best of the latest session that has one, and how it moved
// since the one before that had one.
export function headline(sessions: HistorySession[], m: Measure, scale: Scale | null) {
	if (scale) {
		const [now, then] = sessions.filter((s) => topSend(s.climbs, scale));
		if (!now) return sessions.length ? { session: sessions[0], main: '–', small: '', change: null, step: 'grade' } : null;
		const t = topSend(now.climbs, scale)!;
		const p = then && topSend(then.climbs, scale)!;
		return { session: now, main: t.grade!, small: '', change: p ? rank(t, scale) - rank(p, scale) : null, step: 'grade' };
	}
	const [now, then] = sessions.filter((s) => s.sets.some(done));
	if (!now) return null;
	const b = bestSet(now.sets, m)!;
	const p = then && bestSet(then.sets, m)!;
	const [main, small] = figure(b, m);
	return { session: now, main, small, change: p ? Math.round((setValue(b, m) - setValue(p, m)) * 10) / 10 : null, step: m as string };
}
