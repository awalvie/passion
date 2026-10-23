import { addDays, formatDate, mondayOf } from './dates';
import type { Scale } from './grades';
import type { Climb, RunSummary } from './run';

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
export function hardest(climbs: Climb[], scale: Scale): number | null {
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
