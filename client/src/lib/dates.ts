// Dates are YYYY-MM-DD strings, read and written in UTC, so the phone's own
// time zone cannot move them a day.

export function addDays(date: string, n: number): string {
	const d = new Date(`${date}T00:00:00Z`);
	d.setUTCDate(d.getUTCDate() + n);
	return d.toISOString().slice(0, 10);
}

export function weekday(date: string): string {
	return new Date(`${date}T00:00:00Z`).toLocaleDateString(undefined, { weekday: 'short', timeZone: 'UTC' });
}
