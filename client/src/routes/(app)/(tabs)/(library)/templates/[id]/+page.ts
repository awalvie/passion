import { error } from '@sveltejs/kit';
import { RequestFailed, request } from '$lib/api';
import { loadCentres } from '$lib/centres';
import { addDays } from '$lib/dates';
import { loadToday, type ScheduledDay } from '$lib/plan';
import type { Run } from '$lib/run';
import type { SessionTemplate } from '$lib/template';

export async function load({ params, parent }) {
	try {
		const [template, centres, { runs }] = await Promise.all([
			request<SessionTemplate>('GET', `/api/v1/session-templates/${params.id}`),
			loadCentres(),
			parent()
		]);
		const past = (runs ?? [])
			.filter((r) => r.template === params.id && r.finished_at !== null)
			.sort((a, b) => b.started_at.localeCompare(a.started_at));
		// The Overview's extras only add to the page, so a failed one leaves it out.
		const [last, next] = await Promise.all([
			past[0] ? request<Run>('GET', `/api/v1/runs/${past[0].id}`).catch(() => null) : null,
			loadToday()
				.then((today) =>
					request<{ days: ScheduledDay[] }>('GET', `/api/v1/scheduled-sessions?from=${today}&to=${addDays(today, 27)}`).then(
						({ days }) => ({ today, day: days.find((d) => d.template === params.id && d.status === 'planned') ?? null })
					)
				)
				.catch(() => null)
		]);
		return { template, centres, runs: past, last, next };
	} catch (e) {
		if (e instanceof RequestFailed && e.status === 404) error(404, 'Not found');
		throw e;
	}
}
