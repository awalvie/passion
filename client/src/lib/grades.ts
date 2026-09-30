import { request } from './api';
import type { Account } from './plan';

// The shapes of server/api/grades.go's response.
export type Scale = { system: string; boulder: boolean; grades: string[] };
export type Grades = { scales: Scale[]; ungraded: string[] };

let cached: Promise<{ grades: Grades; account: Account }> | null = null;

// loadGrades fetches the scales and the account's preferred ones once. A
// failure is not kept, so the next call tries again.
export function loadGrades() {
	cached ??= Promise.all([
		request<Grades>('GET', '/api/v1/grades'),
		request<Account>('GET', '/api/v1/accounts/me')
	])
		.then(([grades, account]) => ({ grades, account }))
		.catch((e) => {
			cached = null;
			throw e;
		});
	return cached;
}

// scaleFor is the scale the account grades a boulder or a route in.
export function scaleFor(g: Grades, account: Account, boulder: boolean): Scale | undefined {
	const system = boulder ? account.boulder_grades : account.route_grades;
	return g.scales.find((s) => s.system === system) ?? g.scales.find((s) => s.boulder === boulder);
}
