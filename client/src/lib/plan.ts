import { request } from './api';

// The shape of server/api/scheduled_session.go's list response.
export type ScheduledDay = {
	id: string;
	cycle: string | null;
	template: string;
	local_date: string;
	template_name: string;
	template_icon: string | null;
	run: string | null;
	status: 'done' | 'started' | 'missed' | 'planned';
};

// A goal, with where the person starts, where they end and how they get there.
export type Goal = { text: string; done: boolean; before: string; after: string; how: string };

// The shape of server/api/cycle.go's cycleResponse.
export type Cycle = {
	id: string;
	name: string;
	starts: string;
	ends: string;
	block_days: number;
	block_from: string;
	days: { day: number; template: string }[];
	goals: Goal[];
	notes: string | null;
};

// saveCycle sends the whole cycle, as the server replaces every field. It
// answers the cycle saved and how many days already held their session.
export async function saveCycle(c: Cycle): Promise<{ cycle: Cycle; leftOut: number }> {
	const { id, block_from, ...body } = c;
	const saved = await request<Cycle & { left_out: unknown[] }>('PUT', `/api/v1/cycles/${id}`, body);
	const { left_out, ...cycle } = saved;
	return { cycle, leftOut: left_out.length };
}

// The shape of server/api/auth.go's accountResponse.
export type Account = {
	id: string;
	email: string;
	display_name: string;
	timezone: string;
	boulder_grades: string;
	route_grades: string;
};

// localToday is today's date, written YYYY-MM-DD, in the account's time zone,
// which is the day the server counts a run on.
export function localToday(timezone: string, now = new Date()): string {
	return new Intl.DateTimeFormat('en-CA', { timeZone: timezone }).format(now);
}

export async function loadToday(): Promise<string> {
	const account = await request<Account>('GET', '/api/v1/accounts/me');
	return localToday(account.timezone);
}
