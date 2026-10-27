import { redirect } from '@sveltejs/kit';

// From lg the first session opens beside the list.
export async function load({ parent, url }) {
	const { templates } = await parent();
	if (templates[0] && matchMedia('(min-width: 64rem)').matches) redirect(307, `/templates/${templates[0].id}${url.search}`);
}
