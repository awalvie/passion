import { loadToday, localToday } from '$lib/plan';
import { loadTemplates } from '$lib/template';

export async function load({ url }) {
	// Reading the path loads the list again on every page, so a retire or a copy
	// shows in it. The answer is kept until a change, so a pick asks nothing new.
	void url.pathname;
	const [templates, today] = await Promise.all([
		loadTemplates(),
		// The list still shows when the account cannot be read.
		loadToday().catch(() => localToday(Intl.DateTimeFormat().resolvedOptions().timeZone))
	]);
	return { templates, today };
}
