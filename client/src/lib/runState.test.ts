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

test('removeOption drops one option and lowers the pick count to what is left', async () => {
	const openRun = await openOffline();

	openRun.removeOption('c1', 1);

	const c = items(openRun.run!, 1)[1].choice!;
	expect(c.options.map((o) => o.exercise)).toStrictEqual(['ex-pull', 'ex-row']);
	expect(c.pick).toBe(2);
	expect(urls(openRun.writes)).toStrictEqual([bodyUrl]);
});

test('removeOption keeps a pick count that still fits', async () => {
	const openRun = await openOffline();
	items(openRun.run!, 1)[1].choice!.pick = 1;

	openRun.removeOption('c1', 0);

	expect(items(openRun.run!, 1)[1].choice!.pick).toBe(1);
});

test('removeOption on the last option removes the choice', async () => {
	const openRun = await openOffline();

	openRun.removeOption('c2', 0);

	expect(items(openRun.run!, 1).map((it) => it.step?.id ?? it.choice?.id)).toStrictEqual(['s2', 'c1']);
	expect(urls(openRun.writes)).toStrictEqual([bodyUrl]);
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
