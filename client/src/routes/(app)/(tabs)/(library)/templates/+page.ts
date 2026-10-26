import { loadToday, localToday } from '$lib/plan';
import { loadTemplates } from '$lib/template';

export async function load() {
	const [templates, today] = await Promise.all([
		loadTemplates(),
		// The list still shows when the account cannot be read.
		loadToday().catch(() => localToday(Intl.DateTimeFormat().resolvedOptions().timeZone))
	]);
	return { templates, today };
}
