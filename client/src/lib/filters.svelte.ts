import { afterNavigate, replaceState } from '$app/navigation';

// urlFilters keeps a list page's filters in its query string, so Back from a
// row finds them as they were. Call it while the page component starts.
export function urlFilters<K extends string>(path: string, keys: K[]): Record<K, string> {
	const filters = $state(fromURL());

	// Writing the URL the page opened with would call replaceState before the
	// router is ready, so only a change is written.
	let written = search();
	$effect(() => {
		const next = search();
		if (next === written) return;
		written = next;
		replaceState(`${path}${next}`, {});
	});

	// A link to this same page keeps the component, so the filters must follow
	// the URL it opened.
	afterNavigate(() => {
		Object.assign(filters, fromURL());
		written = search();
	});

	// replaceState is shallow routing, so page.url never holds the filters: on
	// Back it is still the URL the page opened with. The address bar has them.
	function fromURL() {
		const query = new URLSearchParams(location.search);
		return Object.fromEntries(keys.map((k) => [k, query.get(k) ?? ''])) as Record<K, string>;
	}

	function search() {
		const params = new URLSearchParams();
		for (const k of keys) if (filters[k]) params.set(k, filters[k]);
		return params.size ? `?${params}` : '';
	}

	return filters;
}
