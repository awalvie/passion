import { request } from '$lib/api';
import { loadCentres } from '$lib/centres';
import type { Grades } from '$lib/grades';
import type { Account } from '$lib/plan';
import { loadTemplates } from '$lib/template';

export async function load() {
	const [account, grades, centres, templates] = await Promise.all([
		request<Account>('GET', '/api/v1/accounts/me'),
		request<Grades>('GET', '/api/v1/grades'),
		loadCentres(),
		loadTemplates()
	]);
	return { account, grades, centres, templates };
}
