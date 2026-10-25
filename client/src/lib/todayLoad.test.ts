import { beforeEach, expect, test, vi } from 'vitest';
import { mondayOf } from './dates';
import type { ScheduledDay } from './plan';
import type { Run, RunSummary } from './run';

vi.mock('$app/navigation', () => ({ goto: vi.fn(), beforeNavigate: vi.fn() }));

// Items live as own properties, as in a browser, so Object.keys lists them.
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

const answer = (body: unknown, status = 200) => Response.json(body, { status });

// The loaders under test, as a fresh import so the request cache starts empty.
async function loaders() {
	vi.resetModules();
	const layout = await import('../routes/(app)/(tabs)/+layout');
	const page = await import('../routes/(app)/(tabs)/+page');
	return { layout: layout.load as (e: unknown) => Promise<any>, page: page.load as (e: unknown) => Promise<any> };
}

const summary = (id: string, over: Partial<RunSummary> = {}): RunSummary => ({
	id,
	template: null,
	name: id,
	local_date: '2026-10-25',
	started_at: '2026-10-25T09:00:00+01:00',
	timezone: 'Europe/Oslo',
	finished_at: null,
	elapsed_seconds: null,
	place: null,
	...over
});

const day = (id: string, local_date: string, over: Partial<ScheduledDay> = {}): ScheduledDay => ({
	id,
	cycle: null,
	template: `t-${id}`,
	local_date,
	template_name: id,
	template_icon: null,
	run: null,
	status: 'planned',
	...over
});

beforeEach(() => {
	vi.unstubAllGlobals();
	vi.useRealTimers();
	vi.stubGlobal('localStorage', new MemoryStorage());
	vi.stubGlobal('addEventListener', vi.fn());
	vi.stubGlobal('document', { addEventListener: vi.fn(), visibilityState: 'visible' });
});

function at(iso: string) {
	vi.useFakeTimers({ toFake: ['Date'] });
	vi.setSystemTime(new Date(iso));
}

// A table of answers by the start of a path, the longest start winning. A
// function answers from the whole path.
const routes = (table: Record<string, unknown>) => {
	const asked: string[] = [];
	const starts = Object.keys(table).sort((a, b) => b.length - a.length);
	vi.stubGlobal(
		'fetch',
		vi.fn(async (url: string) => {
			asked.push(url);
			const hit = starts.find((prefix) => url.startsWith(prefix));
			if (!hit) return answer({ error: { code: 'not_found', message: 'no such route' } }, 404);
			const body = table[hit];
			return answer(typeof body === 'function' ? body(url) : body);
		})
	);
	return asked;
};

// The tabs layout. Clocks are 2026-10-25 02:30 in Oslo, 30 minutes after the
// clocks went back, so a session begun before midnight is still in its night.
test('the tabs layout keeps an unfinished run live until 04:00, then calls it stale', async () => {
	const { layout } = await loaders();
	const runs = [
		summary('late', { local_date: '2026-10-24', started_at: '2026-10-24T23:30:00+02:00' }),
		summary('forgotten', { local_date: '2026-10-20', started_at: '2026-10-20T17:30:00+02:00' }),
		summary('finished', { local_date: '2026-10-24', finished_at: '2026-10-24T20:00:00+02:00' })
	];
	routes({ '/api/v1/runs': { runs } });

	at('2026-10-25T01:30:00Z');
	let got = await layout({ url: new URL('http://x/') });
	expect(got.live.id).toBe('late');
	expect(got.stale.map((r: RunSummary) => r.id)).toStrictEqual(['forgotten']);
	expect(got.runs).toStrictEqual(runs);

	at('2026-10-25T03:00:00Z');
	got = await (await loaders()).layout({ url: new URL('http://x/') });
	expect(got.live).toBeNull();
	expect(got.stale.map((r: RunSummary) => r.id)).toStrictEqual(['late', 'forgotten']);
});

test('the tabs layout makes the newest run that is not stale the live one', async () => {
	at('2026-10-25T10:00:00Z');
	const { layout } = await loaders();
	routes({
		'/api/v1/runs': {
			runs: [
				summary('today-early', { started_at: '2026-10-25T08:00:00+01:00' }),
				summary('today-late', { started_at: '2026-10-25T10:00:00+01:00' }),
				summary('last-week', { local_date: '2026-10-18', started_at: '2026-10-18T10:00:00+02:00' })
			]
		}
	});

	const got = await layout({ url: new URL('http://x/') });

	expect(got.live.id).toBe('today-late');
	expect(got.stale.map((r: RunSummary) => r.id)).toStrictEqual(['last-week']);
});

test('the tabs layout offers the runs this phone holds when there is no signal', async () => {
	at('2026-10-25T10:00:00Z');
	const { layout } = await loaders();
	const held = (id: string, over: Partial<Run>) =>
		localStorage.setItem(`passion-pending:${id}`, JSON.stringify({ run: { ...summary(id), ...over }, writes: [] }));
	held('forgotten', { local_date: '2026-10-20', started_at: '2026-10-20T17:30:00+02:00' });
	held('today', {});
	held('finished', { finished_at: '2026-10-25T09:30:00+01:00' });
	vi.stubGlobal(
		'fetch',
		vi.fn(async () => {
			throw new TypeError('fetch failed');
		})
	);

	const got = await layout({ url: new URL('http://x/') });

	expect(got.runs).toBeNull();
	expect(got.live.id).toBe('today');
	expect(got.stale.map((r: RunSummary) => r.id)).toStrictEqual(['forgotten']);
});

test('the tabs layout treats a server error as no signal, and shows any other refusal', async () => {
	const { layout } = await loaders();
	vi.stubGlobal('fetch', vi.fn(async () => answer({ error: { code: 'boom', message: 'down' } }, 502)));
	expect((await layout({ url: new URL('http://x/') })).runs).toBeNull();

	vi.stubGlobal('fetch', vi.fn(async () => answer({ error: { code: 'forbidden', message: 'no' } }, 403)));
	await expect((await loaders()).layout({ url: new URL('http://x/') })).rejects.toThrow('no');
});

// Today. 2026-10-25 is a Sunday.
const account = { '/api/v1/accounts/me': { timezone: 'Europe/Oslo' } };
const parent = (over: Record<string, unknown> = {}) => async () => ({ live: null, stale: [], runs: [summary('r')], ...over });

test('Today on a Sunday asks for last week to the end of next, and keeps this week apart', async () => {
	at('2026-10-25T10:00:00Z');
	const { page } = await loaders();
	const days = [
		day('last', '2026-10-13', { status: 'done', run: 'r0' }),
		day('mon', '2026-10-19', { status: 'done', run: 'r1' }),
		day('thu', '2026-10-22', { status: 'missed' }),
		day('sun', '2026-10-25'),
		day('next-mon', '2026-10-26'),
		day('next-wed', '2026-10-28'),
		day('end', '2026-11-01')
	];
	const asked = routes({
		...account,
		'/api/v1/scheduled-sessions': { days },
		'/api/v1/cycles': { cycles: [] },
		'/api/v1/session-templates': { session_templates: [] }
	});

	const got = await page({ parent: parent() });

	expect(got.today).toBe('2026-10-25');
	expect(got.monday).toBe('2026-10-19');
	expect(asked).toContain('/api/v1/scheduled-sessions?from=2026-10-12&to=2026-11-01');
	expect(got.week.map((d: ScheduledDay) => d.id)).toStrictEqual(['mon', 'thu', 'sun']);
	expect(got.days).toHaveLength(7);
	expect(got.next.id).toBe('next-mon');
	// The library list carries every session's details, so none is fetched alone.
	expect(asked.filter((u) => /session-templates\/t-/.test(u))).toStrictEqual([]);
	expect(got.offline).toBe(false);
});

test('Today has no next session when nothing after today is still planned', async () => {
	at('2026-10-25T10:00:00Z');
	const { page } = await loaders();
	routes({
		...account,
		'/api/v1/scheduled-sessions': {
			days: [day('past', '2026-10-27', { status: 'done', run: 'r1' }), day('today', '2026-10-25'), day('yesterday', '2026-10-24', { status: 'missed' })]
		},
		'/api/v1/cycles': { cycles: [] },
		'/api/v1/session-templates': { session_templates: [] }
	});

	expect((await page({ parent: parent() })).next).toBeNull();
});

test('Today on a Monday starts its week that day, and takes the day in the account zone', async () => {
	// 23:30 on Sunday in London is already Monday in Oslo.
	at('2026-10-25T23:30:00Z');
	const { page } = await loaders();
	const asked = routes({
		...account,
		'/api/v1/scheduled-sessions': { days: [] },
		'/api/v1/cycles': { cycles: [] },
		'/api/v1/session-templates': { session_templates: [] }
	});

	const got = await page({ parent: parent() });

	expect(got.today).toBe('2026-10-26');
	expect(got.monday).toBe('2026-10-26');
	expect(asked).toContain('/api/v1/scheduled-sessions?from=2026-10-19&to=2026-11-08');
});

test('Today looks for centres only for someone with no runs yet', async () => {
	at('2026-10-25T10:00:00Z');
	const table = {
		...account,
		'/api/v1/scheduled-sessions': { days: [] },
		'/api/v1/cycles': { cycles: [] },
		'/api/v1/session-templates': { session_templates: [] },
		'/api/v1/centres': { centres: [{ id: 'c', name: 'Gym', sessions: [] }] }
	};
	const { page } = await loaders();
	let asked = routes(table);
	expect((await page({ parent: parent({ runs: [] }) })).centres).toBe(1);
	expect(asked).toContain('/api/v1/centres');

	asked = routes(table);
	expect((await (await loaders()).page({ parent: parent() })).centres).toBeNull();
	expect(asked).not.toContain('/api/v1/centres');

	asked = routes(table);
	expect((await (await loaders()).page({ parent: parent({ runs: null }) })).centres).toBeNull();
	expect(asked).not.toContain('/api/v1/centres');
});

test('Today still opens with no signal, and shows a refusal as an error', async () => {
	at('2026-10-25T10:00:00Z');
	const { page } = await loaders();
	vi.stubGlobal(
		'fetch',
		vi.fn(async () => {
			throw new TypeError('fetch failed');
		})
	);
	const got = await page({ parent: parent({ runs: null }) });
	expect(got).toMatchObject({ offline: true, week: [], days: [], next: null, cycles: [], library: [], centres: null });
	expect(got.monday).toBe(mondayOf(got.today));

	vi.stubGlobal('fetch', vi.fn(async () => answer({ error: { code: 'forbidden', message: 'no' } }, 403)));
	await expect((await loaders()).page({ parent: parent() })).rejects.toThrow('no');
});

test('Today opens the live run from the phone while the plan loads', async () => {
	at('2026-10-25T10:00:00Z');
	const { page } = await loaders();
	const asked = routes({
		...account,
		'/api/v1/scheduled-sessions': { days: [] },
		'/api/v1/cycles': { cycles: [] },
		'/api/v1/session-templates': { session_templates: [] },
		'/api/v1/runs/live-1': { ...summary('live-1'), sections: [], sets: [], climbs: [] }
	});

	await page({ parent: parent({ live: summary('live-1') }) });

	expect(asked).toContain('/api/v1/runs/live-1');
});
