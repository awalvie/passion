import { addDays, daysBetween, formatDate, mondayOf } from './dates';

// The shape of server/api/repeat.go's repeatResponse.
export type Repeat = {
	id: string;
	template: string;
	starts: string;
	ends: string | null;
	every: number;
	unit: 'day' | 'week';
	// 1 for Monday to 7 for Sunday.
	weekdays: number[];
};

export type RepeatRule = Pick<Repeat, 'starts' | 'ends' | 'every' | 'unit' | 'weekdays'>;

export function weekdayOf(date: string): number {
	return ((new Date(`${date}T00:00:00Z`).getUTCDay() + 6) % 7) + 1;
}

// fallsOn says whether the rule falls on date, as server/db/repeat.go's on
// does: every few weeks counts from the Monday of the week it starts in.
export function fallsOn(r: RepeatRule, date: string): boolean {
	if (date < r.starts || (r.ends && date > r.ends)) return false;
	if (r.unit === 'day') return daysBetween(r.starts, date) % r.every === 0;
	return r.weekdays.includes(weekdayOf(date)) && (daysBetween(mondayOf(r.starts), mondayOf(date)) / 7) % r.every === 0;
}

// repeatDays lists the rule's days from from to until, both included.
export function repeatDays(r: RepeatRule, from: string, until: string): string[] {
	const out: string[] = [];
	for (let d = from; d <= until; d = addDays(d, 1)) if (fallsOn(r, d)) out.push(d);
	return out;
}

// nthDay is the rule's nth day, counted from 1, which "after n times" ends on.
// A weekly rule with no weekdays answers null.
export function nthDay(r: RepeatRule, n: number): string | null {
	if (r.unit === 'day') return addDays(r.starts, (n - 1) * r.every);
	const days = [...r.weekdays].sort();
	if (!days.length) return null;
	let count = 0;
	for (let monday = mondayOf(r.starts); ; monday = addDays(monday, 7 * r.every)) {
		for (const w of days) {
			const d = addDays(monday, w - 1);
			if (d >= r.starts && ++count === n) return d;
		}
	}
}

export const weekdayNames =[1, 2, 3, 4, 5, 6, 7].map((w) => formatDate(addDays('2026-10-25', w), { weekday: 'long' }));

function joined(words: string[]): string {
	return words.length < 2 ? words.join('') : `${words.slice(0, -1).join(', ')} and ${words.at(-1)}`;
}

// ruleText says how often it falls: "every Tuesday and Thursday".
export function ruleText(r: Pick<RepeatRule, 'every' | 'unit' | 'weekdays'>): string {
	if (r.unit === 'day') return r.every === 1 ? 'every day' : `every ${r.every} days`;
	const days = joined([...r.weekdays].sort().map((w) => weekdayNames[w - 1]));
	return r.every === 1 ? `every ${days}` : `every ${r.every} weeks on ${days}`;
}
