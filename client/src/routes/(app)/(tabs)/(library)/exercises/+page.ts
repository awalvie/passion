import { matches } from '$lib/exercise';
import { redirect } from '@sveltejs/kit';

// From lg the first exercise the list shows opens beside it.
export async function load({ parent, url }) {
	const { exercises } = await parent();
	const f = Object.fromEntries(url.searchParams);
	const first = exercises.find((e) => (!f.kind || e.kind === f.kind) && matches(e, f));
	if (first && matchMedia('(min-width: 64rem)').matches) redirect(307, `/exercises/${first.id}${url.search}`);
}
