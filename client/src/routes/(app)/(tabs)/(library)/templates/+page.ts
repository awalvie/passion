import { loadTemplates } from '$lib/template';

export async function load() {
	return { templates: await loadTemplates() };
}
