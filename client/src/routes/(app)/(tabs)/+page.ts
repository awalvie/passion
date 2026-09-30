import { request, unreachable } from '$lib/api';
import { localToday, type Account, type ScheduledDay } from '$lib/plan';
import type { SessionTemplate } from '$lib/template';

export async function load() {
	try {
		const account = await request<Account>('GET', '/api/v1/accounts/me');
		const today = localToday(account.timezone);
		const { days } = await request<{ days: ScheduledDay[] }>(
			'GET',
			`/api/v1/scheduled-sessions?from=${today}&to=${today}`
		);
		const templates = await Promise.all(
			[...new Set(days.map((d) => d.template))].map((id) =>
				request<SessionTemplate>('GET', `/api/v1/session-templates/${id}`)
			)
		);
		return { today, days, templates: new Map(templates.map((t) => [t.id, t])), offline: false };
	} catch (e) {
		// With no signal, Today still opens, and the live bar leads to the run.
		if (!unreachable(e)) throw e;
		const today = localToday(Intl.DateTimeFormat().resolvedOptions().timeZone);
		return { today, days: [], templates: new Map<string, SessionTemplate>(), offline: true };
	}
}
