import { beforeEach, expect, test, vi } from 'vitest';
import type { Exercise } from './exercise';
import type { Run, RunChoice, RunStep } from './run';
import { toStep } from './template';

vi.mock('$app/navigation', () => ({ goto: vi.fn() }));

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

const exercise = (id: string, name: string): Exercise => ({
	id,
	shipped: true,
	name,
	kind: 'reps_and_sets',
	notes: null,
	source: null,
	tags: [],
	sets: 3,
	reps: 5,
	set_rest_seconds: 120,
	rep_seconds: null,
	rep_rest_seconds: null,
	prep_seconds: null,
	duration_seconds: null,
	per_side: false,
	ladder: null,
	media: [],
	retired_at: null
});

const pullUp = exercise('ex-pull', 'Pull-up');
const dip = exercise('ex-dip', 'Dip');
const row = exercise('ex-row', 'Row');

const step = (id: string, e: Exercise): RunStep => ({
	...toStep(e),
	id,
	status: null,
	run_notes: null,
	elapsed_seconds: null
});

const choice = (id: string, pick: number, options: Exercise[]): RunChoice => ({
	id,
	name: 'Pull',
	notes: null,
	pick,
	options: options.map(toStep)
});

function newRun(): Run {
	return {
		id: 'run-1',
		template: null,
		scheduled: null,
		name: 'Strength',
		plan: null,
		sections: [
			{ name: 'Warm-up', notes: null, items: [{ step: step('s1', pullUp) }] },
			{
				name: 'Main',
				notes: null,
				items: [
					{ step: step('s2', dip) },
					{ choice: choice('c1', 3, [pullUp, dip, row]) },
					{ choice: choice('c2', 1, [row]) }
				]
			}
		],
		started_at: '2026-10-01T08:00:00Z',
		timezone: 'Europe/Oslo',
		local_date: '2026-10-01',
		finished_at: null,
		elapsed_seconds: null,
		place: null,
		notes: null,
		sets: [],
		climbs: [],
		sleep: null,
		energy: null,
		rpe: null,
		focus: null,
		setting: null,
		went_well: null,
		next_focus: null
	};
}

const bodyUrl = '/api/v1/runs/run-1';

function online() {
	vi.stubGlobal(
		'fetch',
		vi.fn(async (url: string, init: RequestInit) =>
			init.method === 'GET' && url === bodyUrl ? Response.json(newRun()) : new Response(null, { status: 204 })
		)
	);
}

function offline() {
	vi.stubGlobal(
		'fetch',
		vi.fn(async () => {
			throw new TypeError('fetch failed');
		})
	);
}

// A fresh import is a reload: the open run starts empty and only what the
// phone stored comes back.
async function openFresh() {
	vi.resetModules();
	const { openRun } = await import('./runState.svelte');
	await openRun.load('run-1');
	return openRun;
}

// Opens the run from the server, then loses the signal, so every change
// stays queued.
async function openOffline() {
	online();
	const openRun = await openFresh();
	offline();
	return openRun;
}

const urls = (writes: { url: string }[]) => writes.map((w) => w.url);
const stored = () => JSON.parse(localStorage.getItem('passion-pending:run-1')!);
const items = (r: Run, section: number) => r.sections[section].items;

beforeEach(() => {
	vi.unstubAllGlobals();
	vi.stubGlobal('localStorage', new MemoryStorage());
	vi.stubGlobal('addEventListener', vi.fn());
	vi.stubGlobal('document', { addEventListener: vi.fn(), visibilityState: 'visible' });
});

test('removeStep drops the step and the set write still queued for it', async () => {
	const openRun = await openOffline();
	openRun.setSets(openRun.step('s1')!, [{ reps: 5, seconds: null, weight_kg: 10 }]);
	expect(urls(openRun.writes)).toContain(`${bodyUrl}/steps/s1/sets`);

	openRun.removeStep('s1');

	expect(items(openRun.run!, 0)).toStrictEqual([]);
	expect(urls(openRun.writes)).toStrictEqual([bodyUrl]);
	expect(urls(stored().writes)).toStrictEqual([bodyUrl]);
	expect(openRun.writes[0].body).toMatchObject({ sections: [{ items: [] }, { name: 'Main' }] });
});

test('removeStep keeps the set writes of other steps', async () => {
	const openRun = await openOffline();
	openRun.setSets(openRun.step('s2')!, [{ reps: 5, seconds: null, weight_kg: null }]);

	openRun.removeStep('s1');

	expect(urls(openRun.writes).sort()).toStrictEqual([bodyUrl, `${bodyUrl}/steps/s2/sets`]);
});

test('unlog to no sets reopens the step and marks it not done', async () => {
	const openRun = await openOffline();
	const s = openRun.step('s1')!;
	openRun.setSets(s, [{ reps: 5, seconds: null, weight_kg: null }]);
	openRun.finish(s);

	openRun.unlog(s, []);

	expect(s.status).toBeNull();
	expect(s.elapsed_seconds).toBeNull();
	expect(openRun.run!.sets.filter((x) => x.step === 's1')).toStrictEqual([]);
});

test('removeSet drops a done set and lowers the planned count', async () => {
	const openRun = await openOffline();
	const s = openRun.step('s1')!;
	openRun.setSets(s, [
		{ reps: 5, seconds: null, weight_kg: 10 },
		{ reps: 4, seconds: null, weight_kg: 12 }
	]);

	openRun.removeSet(s, 0, 3);

	expect(s.sets).toBe(2);
	expect(openRun.run!.sets.filter((x) => x.step === 's1').map((x) => [x.number, x.reps])).toStrictEqual([[1, 4]]);
});

test('unlog to some sets keeps the step done, reopens it and queues the body before the sets', async () => {
	const openRun = await openOffline();
	const s = openRun.step('s1')!;
	const two = [
		{ reps: 5, seconds: null, weight_kg: 10 },
		{ reps: 4, seconds: null, weight_kg: 12 }
	];
	openRun.setSets(s, two);
	s.elapsed_seconds = 30;

	openRun.unlog(s, two.slice(0, 1));

	expect(s.status).toBe('done');
	expect(s.elapsed_seconds).toBeNull();
	expect(openRun.run!.sets.filter((x) => x.step === 's1').map((x) => x.reps)).toStrictEqual([5]);
	expect(urls(openRun.writes)).toStrictEqual([bodyUrl, `${bodyUrl}/steps/s1/sets`]);
});

test('unlog to no sets reaches the server as a null status in the body, then an empty set list', async () => {
	const openRun = await openOffline();
	const s = openRun.step('s1')!;
	openRun.setSets(s, [{ reps: 5, seconds: null, weight_kg: null }]);
	openRun.unlog(s, []);

	const sent: [string, unknown][] = [];
	vi.stubGlobal(
		'fetch',
		vi.fn(async (url: string, init: RequestInit) => {
			sent.push([url, init.body ? JSON.parse(String(init.body)) : null]);
			return new Response(null, { status: 204 });
		})
	);
	await openRun.settle();

	expect(sent.map(([u]) => u)).toStrictEqual([bodyUrl, `${bodyUrl}/steps/s1/sets`]);
	expect((sent[0][1] as Run).sections[0].items[0].step).toMatchObject({ id: 's1', status: null });
	expect(sent[1][1]).toStrictEqual({ sets: [] });
});

test('removeSet on a per-side step drops both rows of that set and renumbers the rest', async () => {
	const openRun = await openOffline();
	const s = openRun.step('s1')!;
	s.per_side = true;
	s.sets = 3;
	const row = (reps: number) => ({ reps, seconds: null, weight_kg: null });
	openRun.setSets(s, [row(1), row(2), row(3), row(4)]);

	openRun.removeSet(s, 0, 3);

	expect(s.sets).toBe(2);
	expect(openRun.run!.sets.filter((x) => x.step === 's1').map((x) => [x.number, x.reps])).toStrictEqual([
		[1, 3],
		[2, 4]
	]);
});

test('removeSet on a set not logged writes no sets and keeps the logged ones', async () => {
	const openRun = await openOffline();
	const s = openRun.step('s1')!;
	s.sets = 3;
	openRun.setSets(s, [{ reps: 5, seconds: null, weight_kg: null }]);
	openRun.writes = [];

	openRun.removeSet(s, 2, 3);

	expect(s.sets).toBe(2);
	expect(s.status).toBe('done');
	expect(openRun.run!.sets.filter((x) => x.step === 's1')).toHaveLength(1);
	expect(urls(openRun.writes)).toStrictEqual([bodyUrl]);
});

test('removeSet of the only logged set reopens the step and marks it not done', async () => {
	const openRun = await openOffline();
	const s = openRun.step('s1')!;
	openRun.setSets(s, [{ reps: 5, seconds: null, weight_kg: null }]);
	s.elapsed_seconds = 30;

	openRun.removeSet(s, 0, 3);

	expect(s.status).toBeNull();
	expect(s.elapsed_seconds).toBeNull();
	expect(openRun.run!.sets.filter((x) => x.step === 's1')).toStrictEqual([]);
	expect(openRun.writes.find((w) => w.url === `${bodyUrl}/steps/s1/sets`)!.body).toStrictEqual({ sets: [] });
});

test('addSet from a count adds a set to what the page shows', async () => {
	const openRun = await openOffline();
	const s = openRun.step('s1')!;
	s.sets = null;

	openRun.addSet(s, 1);

	expect(s.sets).toBe(2);
});

test('addStep puts the exercise at the end of the section it is given', async () => {
	const openRun = await openOffline();

	openRun.addStep(row, 0);

	const added = items(openRun.run!, 0)[1].step!;
	expect(items(openRun.run!, 0)).toHaveLength(2);
	expect(added).toMatchObject({ exercise: 'ex-row', name: 'Row', status: null, elapsed_seconds: null });
	expect(added.id).not.toBe('s1');
	expect(items(openRun.run!, 1)).toHaveLength(3);
	expect(urls(openRun.writes)).toStrictEqual([bodyUrl]);
});

test('addStep with no section given uses the last one', async () => {
	const openRun = await openOffline();

	openRun.addStep(row);

	expect(items(openRun.run!, 0)).toHaveLength(1);
	expect(items(openRun.run!, 1).at(-1)!.step!.exercise).toBe('ex-row');
});

const order = (r: Run) => r.sections.map((s) => s.items.map((it) => it.step?.id ?? it.choice!.id));

test('moveStep puts the step at the place given and saves the body', async () => {
	const openRun = await openOffline();

	openRun.moveStep('s1', 1, 2);

	expect(order(openRun.run!)).toStrictEqual([[], ['s2', 'c1', 's1', 'c2']]);
	expect(urls(openRun.writes)).toStrictEqual([bodyUrl]);
});

test('shiftStep moves past a choice as one place', async () => {
	const openRun = await openOffline();

	openRun.shiftStep('s2', 1);

	expect(order(openRun.run!)).toStrictEqual([['s1'], ['c1', 's2', 'c2']]);
});

test('shiftStep over the edge of a section moves into the next one', async () => {
	const openRun = await openOffline();

	openRun.shiftStep('s2', -1);
	expect(order(openRun.run!)).toStrictEqual([['s1', 's2'], ['c1', 'c2']]);

	openRun.shiftStep('s1', 1);
	openRun.shiftStep('s1', 1);
	expect(order(openRun.run!)).toStrictEqual([['s2'], ['s1', 'c1', 'c2']]);
});

test('shiftStep goes into an empty section next to it', async () => {
	const openRun = await openOffline();
	openRun.addSection('Cool-down');

	openRun.shiftStep('s2', -1);
	openRun.shiftStep('s1', 1);
	openRun.shiftStep('s1', 1);
	openRun.shiftStep('s1', 1);
	openRun.shiftStep('s1', 1);
	openRun.shiftStep('s1', 1);

	expect(order(openRun.run!)).toStrictEqual([['s2'], ['c1', 'c2'], ['s1']]);
});

test('dropStep counts the place before the step leaves it', async () => {
	const openRun = await openOffline();

	openRun.dropStep('s2', 1, 2);
	expect(order(openRun.run!)).toStrictEqual([['s1'], ['c1', 's2', 'c2']]);

	openRun.dropStep('s2', 1, 3);
	expect(order(openRun.run!)).toStrictEqual([['s1'], ['c1', 'c2', 's2']]);

	openRun.dropStep('s2', 0, 0);
	expect(order(openRun.run!)).toStrictEqual([['s2', 's1'], ['c1', 'c2']]);
});

test('dropStep on its own place writes nothing', async () => {
	const openRun = await openOffline();

	openRun.dropStep('s2', 1, 0);
	openRun.dropStep('s2', 1, 1);

	expect(openRun.writes).toStrictEqual([]);
});

test('addSection adds an empty section at the end', async () => {
	const openRun = await openOffline();

	openRun.addSection('Cool-down');

	expect(openRun.run!.sections.at(-1)).toStrictEqual({ name: 'Cool-down', notes: null, items: [] });
	expect(openRun.writes[0].body).toMatchObject({ sections: [{}, {}, { name: 'Cool-down' }] });
});

test('a change made offline survives a reload and is sent when the signal is back', async () => {
	const first = await openOffline();
	first.addSection('Cool-down');
	first.removeStep('s1');

	online();
	const fetch = globalThis.fetch as ReturnType<typeof vi.fn>;
	const openRun = await openFresh();

	expect(openRun.run!.sections.map((s) => s.name)).toStrictEqual(['Warm-up', 'Main', 'Cool-down']);
	expect(items(openRun.run!, 0)).toStrictEqual([]);
	expect(fetch.mock.calls.some(([, init]) => init.method === 'GET')).toBe(false);

	await vi.waitFor(() => expect(openRun.writes).toStrictEqual([]));
	const [url, init] = fetch.mock.calls[0];
	expect([url, init.method]).toStrictEqual([bodyUrl, 'PUT']);
	expect(JSON.parse(init.body).sections.map((s: { name: string }) => s.name)).toStrictEqual([
		'Warm-up',
		'Main',
		'Cool-down'
	]);
	expect(stored().writes).toStrictEqual([]);
});

test('putClimb counts a graded climb with any send as sent, and an ungraded one never', async () => {
	const openRun = await openOffline();
	const s = openRun.step('s1')!;
	s.kind = 'climbing';
	const climb = {
		step: 's1',
		position: 0,
		discipline: 'boulder' as const,
		setting: 'indoor' as const,
		board: null,
		rope_style: null,
		grade: '6c',
		grade_system: 'font',
		goes: ['attempt' as const, 'send' as const, 'attempt' as const],
		seconds: null,
		stars: null,
		focus: null,
		notes: null
	};

	openRun.putClimb(s, 'c-1', climb);
	openRun.putClimb(s, 'c-2', { ...climb, goes: ['attempt'] });
	openRun.putClimb(s, 'c-3', { ...climb, grade: 'Traverse', grade_system: null, goes: ['send'] });

	expect(openRun.run!.climbs.map((c) => c.sent)).toStrictEqual([true, false, false]);
	expect(s.status).toBe('done');
});

test('removeClimb of the last climb reopens the step and queues the body before the delete', async () => {
	const openRun = await openOffline();
	const s = openRun.step('s1')!;
	s.kind = 'climbing';
	const climb = {
		step: 's1',
		position: 0,
		discipline: 'boulder' as const,
		setting: 'indoor' as const,
		board: null,
		rope_style: null,
		grade: '6c',
		grade_system: 'font',
		goes: ['send' as const],
		seconds: null,
		stars: null,
		focus: null,
		notes: null
	};
	openRun.putClimb(s, 'c-1', climb);
	openRun.putClimb(s, 'c-2', { ...climb, position: 1 });

	openRun.removeClimb(s, 'c-1');
	expect(s.status).toBe('done');

	openRun.removeClimb(s, 'c-2');
	expect(s.status).toBeNull();
	expect(openRun.run!.climbs).toStrictEqual([]);
	expect(urls(openRun.writes).slice(-2)).toStrictEqual([bodyUrl, `${bodyUrl}/climbs/c-2`]);
});
