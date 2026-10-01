import { request } from '$lib/api';
import type { Grades } from '$lib/grades';
import type { Account } from '$lib/plan';

export async function load() {
	const [account, grades] = await Promise.all([
		request<Account>('GET', '/api/v1/accounts/me'),
		request<Grades>('GET', '/api/v1/grades')
	]);
	return { account, grades };
}
