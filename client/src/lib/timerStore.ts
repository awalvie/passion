import type { Clock } from './timeline';

// A run's clocks live in localStorage as end times, not counters, so a locked
// phone or a reload loses nothing.
export type Timers = {
	rest: { endsAt: number } | null;
	timed?: Timed | null;
};

// A running timed-reps step: its clock, the blocks cut short, the weight each
// of its rows logs, and the time added to its phases.
export type Timed = {
	step: string;
	clock: Clock;
	short: Record<number, number>;
	weight: number | null;
	extra?: Record<number, number>;
};

const key = (runId: string) => `passion-timer:${runId}`;

export function readTimers(runId: string): Timers {
	try {
		const raw = localStorage.getItem(key(runId));
		return raw ? (JSON.parse(raw) as Timers) : { rest: null };
	} catch {
		return { rest: null };
	}
}

export function writeTimers(runId: string, t: Timers) {
	try {
		localStorage.setItem(key(runId), JSON.stringify(t));
	} catch {
		/* the clock in memory still runs */
	}
}

export function formatClock(seconds: number): string {
	const s = Math.max(0, Math.ceil(seconds));
	return `${Math.floor(s / 60)}:${String(s % 60).padStart(2, '0')}`;
}

// sessionClock writes a session's time as m:ss, or h:mm:ss past an hour.
export function sessionClock(seconds: number): string {
	const s = Math.max(0, Math.floor(seconds));
	return s < 3600 ? formatClock(s) : `${Math.floor(s / 3600)}:${formatClock(s % 3600).padStart(5, '0')}`;
}
