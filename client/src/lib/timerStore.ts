// A run's clocks live in localStorage as end times, not counters, so a locked
// phone or a reload loses nothing.
export type Timers = { rest: { endsAt: number } | null };

const key = (runId: string) => `passion-timer:${runId}`;

export function readTimers(runId: string): Timers {
	const raw = localStorage.getItem(key(runId));
	return raw ? (JSON.parse(raw) as Timers) : { rest: null };
}

export function writeTimers(runId: string, t: Timers) {
	localStorage.setItem(key(runId), JSON.stringify(t));
}

export function formatClock(seconds: number): string {
	const s = Math.max(0, Math.ceil(seconds));
	return `${Math.floor(s / 60)}:${String(s % 60).padStart(2, '0')}`;
}
