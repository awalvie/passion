// Dates are YYYY-MM-DD strings, read and written in UTC, so the phone's own
// time zone cannot move them a day.

export function addDays(date: string, n: number): string {
	const d = new Date(`${date}T00:00:00Z`);
	d.setUTCDate(d.getUTCDate() + n);
	return d.toISOString().slice(0, 10);
}

export const weekdayLetters = ['M', 'T', 'W', 'T', 'F', 'S', 'S'];

// byDate groups items under the date each one names.
export function byDate<T>(items: T[], date: (t: T) => string): Map<string, T[]> {
	const m = new Map<string, T[]>();
	for (const t of items) {
		const key = date(t);
		const list = m.get(key);
		if (list) list.push(t);
		else m.set(key, [t]);
	}
	return m;
}

export function mondayOf(date: string): string {
	const day = new Date(`${date}T00:00:00Z`).getUTCDay();
	return addDays(date, -((day + 6) % 7));
}

// cycleWeek says which week of a cycle a date falls in, counted from its start.
export function cycleWeek(starts: string, ends: string, date: string): { week: number; of: number } {
	return { week: Math.floor(daysBetween(starts, date) / 7) + 1, of: Math.ceil((daysBetween(starts, ends) + 1) / 7) };
}

// gridWeeks lists the Monday-to-Sunday weeks that hold from..to, each as its
// seven dates.
export function gridWeeks(from: string, to: string): string[][] {
	const weeks: string[][] = [];
	for (let monday = mondayOf(from); monday <= to; monday = addDays(monday, 7)) {
		weeks.push(Array.from({ length: 7 }, (_, i) => addDays(monday, i)));
	}
	return weeks;
}

// yearSpan places from..to, both included, on a year as fractions of it, or
// answers null when the span misses the year.
export function yearSpan(from: string, to: string, year: number): { left: number; width: number } | null {
	const start = Date.UTC(year, 0, 1);
	const end = Date.UTC(year + 1, 0, 1);
	const a = Math.max(Date.parse(from), start);
	const b = Math.min(Date.parse(to) + 86_400_000, end);
	if (b <= a) return null;
	return { left: (a - start) / (end - start), width: (b - a) / (end - start) };
}

export function formatDate(date: string, o: Intl.DateTimeFormatOptions): string {
	return new Date(`${date}T00:00:00Z`).toLocaleDateString(undefined, { ...o, timeZone: 'UTC' });
}

// timeOfDay is an instant as 16:26 in the given zone, so a session logged
// abroad keeps the clock time it had there.
export function timeOfDay(at: string, timeZone: string): string {
	return new Date(at).toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit', hourCycle: 'h23', timeZone });
}

export function daysBetween(from: string, to: string): number {
	return Math.round((Date.parse(to) - Date.parse(from)) / 86_400_000);
}

// shortDate is a date as 7 Mar.
export function shortDate(date: string): string {
	return formatDate(date, { day: 'numeric', month: 'short' });
}

export function weekday(date: string): string {
	return formatDate(date, { weekday: 'short' });
}

// fullDate is a date as Thursday 1 October, in the reader's order.
export function fullDate(date: string): string {
	return formatDate(date, { weekday: 'long', day: 'numeric', month: 'long' });
}

// dayDate is a date as Thu 1 Oct, in the reader's order.
export function dayDate(date: string): string {
	return formatDate(date, { weekday: 'short', day: 'numeric', month: 'short' });
}

// monthSpan is October – November, or October when both fall in it.
export function monthSpan(from: string, to: string): string {
	const a = formatDate(from, { month: 'long' });
	const b = formatDate(to, { month: 'long' });
	return a === b ? a : `${a} – ${b}`;
}
