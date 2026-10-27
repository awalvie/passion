import { redirect } from '@sveltejs/kit';

// From lg the first exercise of the open type opens beside the list.
export async function load({ parent, url }) {
	const { exercises } = await parent();
	const kind = url.searchParams.get('kind');
	const first = exercises.find((e) => !kind || e.kind === kind);
	if (first && matchMedia('(min-width: 64rem)').matches) redirect(307, `/exercises/${first.id}${url.search}`);
}
