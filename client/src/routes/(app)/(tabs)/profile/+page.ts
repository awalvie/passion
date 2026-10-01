import { request } from '$lib/api';
import { loadCentres } from '$lib/centres';
import type { Grades } from '$lib/grades';
import type { Account } from '$lib/plan';
import type { SessionTemplate } from '$lib/template';

export async function load() {
	const [account, grades, centres, { session_templates }] = await Promise.all([
		request<Account>('GET', '/api/v1/accounts/me'),
		request<Grades>('GET', '/api/v1/grades'),
		loadCentres(),
		request<{ session_templates: SessionTemplate[] }>('GET', '/api/v1/session-templates')
	]);
	return { account, grades, centres, templates: session_templates };
}
