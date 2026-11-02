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

// A period that has ended, with the goals that counted in it.
export type PastPeriod = {
	key: string;
	title: string;
	note: string;
	from: string;
	to: string;
	// A removed goal can no longer be ticked.
	goals: { id: string; text: string; done: boolean; removed: boolean }[];
};

// pastPeriods is every period that has ended, the latest first. A goal counts
// in each of its periods from the one it was set in, until it was removed,
// and in one it was met in before then. A met goal reads as it did when ticked.
export function pastPeriods(goals: PeriodGoal[], today: string): PastPeriod[] {
	const year = today.slice(0, 4);
	const out = new Map<string, PastPeriod>();
	const add = (key: string, title: string, note: string, from: string, to: string, g: PeriodGoal) => {
		if (!out.has(key)) out.set(key, { key, title, note, from, to, goals: [] });
		const tick = g.ticks.find((t) => t.period_start === from);
		out.get(key)!.goals.push({ id: g.id, text: tick?.text ?? g.text, done: !!tick, removed: !!g.removed });
	};
	for (const g of goals) {
		const counts = (from: string, to: string) =>
			to < today && (!g.removed || to < g.removed || g.ticks.some((t) => t.period_start === from));
		if (g.span) {
			const s = g.span;
			if (!counts(s.starts, s.ends)) continue;
			const days = `${shortDate(s.starts)} – ${shortDate(s.ends)}`;
			add(`span-${s.id}`, s.name || days, s.name ? days : `${daysBetween(s.starts, s.ends) + 1} days`, s.starts, s.ends, g);
			continue;
		}
		const period = g.period as RepeatPeriod;
		// A removed goal's periods end with the one it went in, met or not.
		const last = g.removed ? periodOf(period, g.removed).to : today;
		for (let { from, to } = periodOf(period, g.starts); to <= last; ({ from, to } = periodOf(period, addDays(to, 1)))) {
			if (!counts(from, to)) continue;
			if (period === 'week') {
				const title = `${shortDate(from)} – ${shortDate(to)}${to.startsWith(year) ? '' : `, ${to.slice(0, 4)}`}`;
				add(`week-${from}`, title, 'Week', from, to, g);
			} else if (period === 'month') {
				const title = formatDate(from, from.startsWith(year) ? { month: 'long' } : { month: 'long', year: 'numeric' });
				add(`month-${from}`, title, 'Month', from, to, g);
			} else add(`year-${from}`, from.slice(0, 4), 'Year', from, to, g);
		}
	}
	return [...out.values()].sort((a, b) => b.to.localeCompare(a.to) || a.key.localeCompare(b.key));
}
