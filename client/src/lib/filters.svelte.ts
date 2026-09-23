import { replaceState } from '$app/navigation';
import { page } from '$app/state';

// urlFilters keeps a list page's filters in its query string, so Back from a
// row finds them as they were. Call it while the page component starts.
export function urlFilters<K extends string>(path: string, keys: K[]): Record<K, string> {
	const query = page.url.searchParams;
	const filters = $state(Object.fromEntries(keys.map((k) => [k, query.get(k) ?? ''])) as Record<K, string>);

	// Writing the URL the page opened with would call replaceState before the
	// router is ready, so only a change is written.
	let written = search();
	$effect(() => {
		const next = search();
		if (next === written) return;
		written = next;
		replaceState(`${path}${next}`, {});
	});

	function search() {
		const params = new URLSearchParams();
		for (const k of keys) if (filters[k]) params.set(k, filters[k]);
		return params.size ? `?${params}` : '';
	}

	return filters;
}
