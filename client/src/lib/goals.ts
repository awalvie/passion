import { request } from './api';
import { addDays, daysBetween, formatDate, mondayOf, shortDate } from './dates';

export type Period = 'week' | 'month' | 'year' | 'custom';
export type RepeatPeriod = Exclude<Period, 'custom'>;

// The shape of server/api/goal.go's goalResponse.
export type PeriodGoal = {
	id: string;
	text: string;
	period: Period;
	starts: string;
	// The day it was removed. It still counts in the periods that ended before.
	removed: string | null;
	span: GoalSpan | null;
	ticks: { period_start: string; text: string }[];
};

// A custom goal's days, shared by every goal in them. An empty name means
// they are known by their dates.
export type GoalSpan = { id: string; name: string; starts: string; ends: string };

export async function loadGoals(): Promise<PeriodGoal[]> {
	return (await request<{ goals: PeriodGoal[] }>('GET', '/api/v1/goals')).goals;
}

// periodOf is the first and last day of the period holding date.
export function periodOf(period: RepeatPeriod, date: string): { from: string; to: string } {
	if (period === 'week') {
		const from = mondayOf(date);
		return { from, to: addDays(from, 6) };
	}
	const [y, m] = date.split('-').map(Number);
	if (period === 'year') return { from: `${y}-01-01`, to: `${y}-12-31` };
	// Day 0 of the next month is this month's last.
	return { from: date.slice(0, 8) + '01', to: new Date(Date.UTC(y, m, 0)).toISOString().slice(0, 10) };
}

// goalPeriod is the goal's period that holds today, or a custom goal's only one.
export const goalPeriod = (g: PeriodGoal, today: string) =>
	g.span ? { from: g.span.starts, to: g.span.ends } : periodOf(g.period as RepeatPeriod, today);

export const ticked = (g: PeriodGoal, today: string) => g.ticks.some((t) => t.period_start === goalPeriod(g, today).from);

// current is the goals still running: not removed, and not on days that ended.
export const current = (goals: PeriodGoal[], today: string) =>
	goals.filter((g) => !g.removed && (!g.span || g.span.ends >= today));

// left says how long the span from today to its last day has to run.
export function left(to: string, today: string): string {
	const n = daysBetween(today, to);
	return n === 0 ? 'ends today' : n === 1 ? 'ends tomorrow' : `ends in ${n} days`;
}

// through is how far through from–to today is, from 0 before it starts to 1.
export const through = (from: string, to: string, today: string) =>
	Math.min(1, Math.max(0, (daysBetween(from, today) + 1) / (daysBetween(from, to) + 1)));

// A period's heading, and the line under it.
export function periodTitle(period: RepeatPeriod, today: string): { title: string; note: string } {
	const { from, to } = periodOf(period, today);
	if (period === 'week') return { title: 'This week', note: `${shortDate(from)} – ${shortDate(to)} · ${left(to, today)}` };
	if (period === 'month') return { title: formatDate(today, { month: 'long' }), note: `This month · ${left(to, today)}` };
	return { title: today.slice(0, 4), note: `This year · ${left(to, today)}` };
}

// A span's heading, and the line under it.
export function spanTitle(span: GoalSpan, today: string): { title: string; note: string } {
	const days = `${shortDate(span.starts)} – ${shortDate(span.ends)}`;
	const n = daysBetween(today, span.starts);
	const when = n > 1 ? `starts in ${n} days` : n === 1 ? 'starts tomorrow' : left(span.ends, today);
	if (span.name) return { title: span.name, note: `${days} · ${when}` };
	return { title: days, note: `${daysBetween(span.starts, span.ends) + 1} days · ${when}` };
}
