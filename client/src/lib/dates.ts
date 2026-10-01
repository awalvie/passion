// Dates are YYYY-MM-DD strings, read and written in UTC, so the phone's own
// time zone cannot move them a day.

export function addDays(date: string, n: number): string {
	const d = new Date(`${date}T00:00:00Z`);
	d.setUTCDate(d.getUTCDate() + n);
	return d.toISOString().slice(0, 10);
}

export function mondayOf(date: string): string {
	const day = new Date(`${date}T00:00:00Z`).getUTCDay();
	return addDays(date, -((day + 6) % 7));
}

// cycleWeek says which week of a cycle a date falls in, counted from its start.
export function cycleWeek(starts: string, ends: string, date: string): { week: number; of: number } {
	const days = (a: string, b: string) => Math.round((Date.parse(b) - Date.parse(a)) / 86_400_000);
	return { week: Math.floor(days(starts, date) / 7) + 1, of: Math.ceil((days(starts, ends) + 1) / 7) };
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

export function weekday(date: string): string {
	return new Date(`${date}T00:00:00Z`).toLocaleDateString(undefined, { weekday: 'short', timeZone: 'UTC' });
}
