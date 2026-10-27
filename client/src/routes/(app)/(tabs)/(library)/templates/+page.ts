import { matches } from '$lib/exercise';
import { redirect } from '@sveltejs/kit';

// From lg the first session the list shows opens beside it.
export async function load({ parent, url }) {
	const { templates } = await parent();
	const first = templates.find((t) => matches(t, Object.fromEntries(url.searchParams)));
	if (first && matchMedia('(min-width: 64rem)').matches) redirect(307, `/templates/${first.id}${url.search}`);
}
