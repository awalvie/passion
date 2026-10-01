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

export function weekday(date: string): string {
	return new Date(`${date}T00:00:00Z`).toLocaleDateString(undefined, { weekday: 'short', timeZone: 'UTC' });
}
