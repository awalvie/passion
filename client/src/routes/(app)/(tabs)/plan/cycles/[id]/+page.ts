import { request } from '$lib/api';
import type { Cycle } from '$lib/plan';
import type { SessionTemplate } from '$lib/template';

export async function load({ params }) {
	const [cycle, { session_templates }] = await Promise.all([
		request<Cycle>('GET', `/api/v1/cycles/${params.id}`),
		request<{ session_templates: SessionTemplate[] }>('GET', '/api/v1/session-templates')
	]);
	return { cycle, templates: session_templates };
}
