import { afterNavigate, replaceState } from '$app/navigation';

// urlFilters keeps a list's filters in the query string of the page it is on,
// so Back from a row finds them as they were. Call it while the component
// starts.
export function urlFilters<K extends string>(keys: K[]): Record<K, string> {
	const filters = $state(fromURL());

	// Writing the URL the page opened with would call replaceState before the
	// router is ready, so only a change is written.
	let written = search();
	$effect(() => {
		const next = search();
		if (next === written) return;
		written = next;
		replaceState(`${location.pathname}${next}`, {});
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

	// The page's own parameters stay as they are.
	function search() {
		const params = new URLSearchParams(location.search);
		for (const k of keys) {
			if (filters[k]) params.set(k, filters[k]);
			else params.delete(k);
		}
		return params.size ? `?${params}` : '';
	}

	return filters;
}

// filterQuery is the filters as a query string, for a link that keeps them.
export function filterQuery(filters: Record<string, string>) {
	const params = new URLSearchParams(Object.entries(filters).filter(([, v]) => v));
	return params.size ? `?${params}` : '';
}
