import { addDays, formatDate, mondayOf } from './dates';
import type { Scale } from './grades';
import type { Climb, HistorySession, RunSummary, SetFields } from './run';

export type DatedClimb = Climb & { run: string; local_date: string };

export type Range = '10w' | '6m' | '1y';
export const ranges: { range: Range; label: string }[] = [
	{ range: '10w', label: '10 weeks' },
	{ range: '6m', label: '6 months' },
	{ range: '1y', label: 'Year' }
];

// tick is the month's name on a chart's time axis, set only on the bucket a
// month starts in.
export type Bucket = { from: string; to: string; tick: string };

// buckets splits a range ending today into weeks, or into months for a year,
// oldest first, the last one holding today.
export function buckets(range: Range, today: string): Bucket[] {
	if (range === '1y') {
		return Array.from({ length: 12 }, (_, i) => {
			const d = new Date(`${today.slice(0, 7)}-01T00:00:00Z`);
			d.setUTCMonth(d.getUTCMonth() - (11 - i));
			const from = d.toISOString().slice(0, 10);
			d.setUTCMonth(d.getUTCMonth() + 1);
			return { from, to: addDays(d.toISOString().slice(0, 10), -1), tick: formatDate(from, { month: 'short' }) };
		});
	}
	const n = range === '10w' ? 10 : 26;
	const last = mondayOf(today);
	return Array.from({ length: n }, (_, i) => {
		const from = addDays(last, -7 * (n - 1 - i));
		const to = addDays(from, 6);
		return { from, to, tick: from.slice(0, 7) === to.slice(0, 7) && !from.endsWith('-01') ? '' : formatDate(to, { month: 'short' }) };
	});
}

const within = (b: Bucket, date: string) => b.from <= date && date <= b.to;

// hours is the logged training time in each bucket.
export function hours(runs: RunSummary[], bs: Bucket[]): number[] {
	return bs.map((b) => runs.reduce((h, r) => h + (within(b, r.local_date) ? (r.elapsed_seconds ?? 0) / 3600 : 0), 0));
}

export const isBoulder = (c: Pick<Climb, 'discipline'>) => c.discipline === 'boulder';

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
