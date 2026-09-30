import { request } from '$lib/api';
import type { SessionTemplate } from '$lib/template';

export async function load() {
	const { session_templates } = await request<{ session_templates: SessionTemplate[] }>(
		'GET',
		'/api/v1/session-templates'
	);
	return { templates: session_templates };
}
