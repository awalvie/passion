import { beforeEach, expect, test, vi } from 'vitest';
import type { RunSummary } from './run';

vi.mock('$app/navigation', () => ({ goto: vi.fn(), beforeNavigate: vi.fn() }));

class MemoryStorage {
	[k: string]: unknown;
	getItem(k: string): string | null {
		return Object.hasOwn(this, k) ? String(this[k]) : null;
	}
	setItem(k: string, v: string) {
		this[k] = String(v);
	}
	removeItem(k: string) {
		delete this[k];
	}
}

// A fresh import, so the grades and request caches start empty.
async function loader() {
	vi.resetModules();
	return (await import('../routes/(app)/(tabs)/history/+page')).load as () => Promise<any>;
}

const summary = (id: string, local_date: string, over: Partial<RunSummary> = {}): RunSummary => ({
	id,
	template: null,
	name: id,
	local_date,
	started_at: `${local_date}T09:00:00+01:00`,
	timezone: 'Europe/Oslo',
	finished_at: `${local_date}T10:00:00+01:00`,
	elapsed_seconds: 3600,
	place: null,
	...over
});

// The server's answers. A path in failing answers 500.
function server(runs: RunSummary[], failing: string[] = []) {
	const asked: string[] = [];
	vi.stubGlobal(
		'fetch',
		vi.fn(async (url: string) => {
			asked.push(url);
			if (failing.some((p) => url.startsWith(p))) return Response.json({ error: { code: 'internal', message: 'boom' } }, { status: 500 });
			if (url.startsWith('/api/v1/runs')) return Response.json({ runs });
			if (url.startsWith('/api/v1/accounts/me')) return Response.json({ timezone: 'Europe/Oslo' });
			if (url.startsWith('/api/v1/grades')) return Response.json({ scales: [], ungraded: [] });
			if (url.startsWith('/api/v1/climbs')) return Response.json({ climbs: [{ run: 'r' }] });
			if (url.startsWith('/api/v1/sets')) return Response.json({ sets: [{ run: 'r' }] });
			return Response.json({ error: { code: 'not_found', message: 'no such route' } }, { status: 404 });
		})
	);
	return asked;
}

beforeEach(() => {
	vi.unstubAllGlobals();
	vi.useFakeTimers({ toFake: ['Date'] });
	vi.setSystemTime(new Date('2026-11-06T12:00:00Z'));
	vi.stubGlobal('localStorage', new MemoryStorage());
	vi.stubGlobal('addEventListener', vi.fn());
	vi.stubGlobal('document', { addEventListener: vi.fn(), visibilityState: 'visible' });
});

test('the facts for the rows are fetched from the oldest run to today, even when that run is unfinished', async () => {
	const asked = server([summary('new', '2026-11-05'), summary('mid', '2026-09-01'), summary('old', '2026-03-02', { finished_at: null })]);

	const data = await (await loader())();
	await data.done;

	expect(asked).toContain('/api/v1/climbs?from=2026-03-02&to=2026-11-06');
	expect(asked).toContain('/api/v1/sets?from=2026-03-02&to=2026-11-06');
	expect(data.runs.map((r: RunSummary) => r.id)).toStrictEqual(['new', 'mid']);
});

test('with no runs the facts are asked for today alone', async () => {
	const asked = server([]);

	const data = await (await loader())();
	await data.done;

	expect(asked).toContain('/api/v1/climbs?from=2026-11-06&to=2026-11-06');
	expect(asked).toContain('/api/v1/sets?from=2026-11-06&to=2026-11-06');
});

test('the facts for the rows arrive on their own after the list is ready', async () => {
	server([summary('a', '2026-11-05')]);

	const data = await (await loader())();

	expect(data.runs).toHaveLength(1);
	expect(await data.done).toStrictEqual({ climbs: [{ run: 'r' }], sets: [{ run: 'r' }] });
});

test.each(['/api/v1/climbs?from=2026-11-05', '/api/v1/sets'])('a failed %s fetch leaves the list loading and gives no facts, so the page keeps what it had', async (failing) => {
	server([summary('a', '2026-11-05')], [failing]);

	const data = await (await loader())();

	expect(data.runs).toHaveLength(1);
	expect(await data.done).toBeNull();
});

test('a run dated after today, logged in a zone ahead, is inside the range', async () => {
	const asked = server([summary('ahead', '2026-11-07'), summary('old', '2026-10-01')]);

	const data = await (await loader())();
	await data.done;

	expect(asked).toContain('/api/v1/climbs?from=2026-10-01&to=2026-11-07');
});
