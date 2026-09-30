// The shape of server/api/scheduled_session.go's list response.
export type ScheduledDay = {
	id: string;
	cycle: string | null;
	template: string;
	local_date: string;
	template_name: string;
	run: string | null;
	status: 'done' | 'started' | 'missed' | 'planned';
};

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
