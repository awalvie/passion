import { RequestFailed, request } from './api';
import { stepsOf, toRunBody, type Run, type RunStep, type SetFields } from './run';

// A write waiting for the server. A newer write to the same url replaces an
// older one, since each carries the whole list or the whole run.
type Write = { seq: number; method: 'PUT' | 'DELETE'; url: string; body?: unknown };

type Stored = { run: Run; writes: Write[] };

const storageKey = (id: string) => `passion-pending:${id}`;

// The run the person has open. The session screen and the player share it.
// Every change lands here first and reaches the server through the queue, so
// a gym with no signal loses nothing.
class OpenRun {
	run = $state<Run | null>(null);
	writes = $state<Write[]>([]);
	// A write the server refused. It is dropped, and this says why.
	refused = $state('');
	// True while writes wait after a send that did not reach the server.
	stalled = $state(false);
	#seq = 0;
	#flushing = false;

	async load(id: string) {
		const stored = localStorage.getItem(storageKey(id));
		if (stored) {
			const { run, writes } = JSON.parse(stored) as Stored;
			this.run = run;
			this.writes = writes;
			this.#seq = Math.max(0, ...writes.map((w) => w.seq));
			void this.flush();
			return;
		}
		this.run = await request<Run>('GET', `/api/v1/runs/${id}`);
		this.writes = [];
	}

	step(id: string): RunStep | undefined {
		return this.run ? stepsOf(this.run).find((s) => s.id === id) : undefined;
	}

	// setSets replaces a step's sets. The server marks the step done at its
	// first set, and this mirrors that, so the next body write agrees.
	setSets(step: RunStep, sets: SetFields[]) {
		const run = this.run!;
		run.sets = [
			...run.sets.filter((s) => s.step !== step.id),
			...sets.map((s, i) => ({ ...s, step: step.id, number: i + 1, exercise: step.exercise }))
		];
		if (sets.length && step.status === null) step.status = 'done';
		this.#queue('PUT', `/api/v1/runs/${run.id}/steps/${step.id}/sets`, { sets });
	}

	// saveBody sends the whole run after a change to its sections or fields.
	saveBody() {
		const run = this.run!;
		this.#queue('PUT', `/api/v1/runs/${run.id}`, toRunBody($state.snapshot(run) as Run));
	}

	// skip drops what the step logged, as the server does, and any set write
	// still waiting, which the server would refuse.
	skip(step: RunStep, elapsed: number) {
		const run = this.run!;
		step.status = 'skipped';
		step.elapsed_seconds = elapsed;
		run.sets = run.sets.filter((s) => s.step !== step.id);
		this.writes = this.writes.filter((w) => w.url !== `/api/v1/runs/${run.id}/steps/${step.id}/sets`);
		this.saveBody();
	}

	#queue(method: Write['method'], url: string, body?: unknown) {
		this.writes = [...this.writes.filter((w) => w.url !== url), { seq: ++this.#seq, method, url, body }];
		this.#store();
		void this.flush();
	}

	#store() {
		const run = this.run;
		if (!run) return;
		if (this.writes.length) {
			const stored: Stored = { run: $state.snapshot(run) as Run, writes: $state.snapshot(this.writes) };
			localStorage.setItem(storageKey(run.id), JSON.stringify(stored));
		} else {
			localStorage.removeItem(storageKey(run.id));
		}
	}

	// flush sends one write at a time, the body first, so a step the body adds
	// exists before its sets arrive. It stops at the first write that could
	// not reach the server and tries again on the next trigger.
	async flush() {
		if (this.#flushing || !this.run) return;
		this.#flushing = true;
		const runId = this.run.id;
		const bodyUrl = `/api/v1/runs/${runId}`;
		try {
			while (this.writes.length && this.run?.id === runId) {
				const w = this.writes.find((x) => x.url === bodyUrl) ?? this.writes[0];
				const { seq, method, url, body } = $state.snapshot(w);
				try {
					await request(method, url, body, AbortSignal.timeout(10_000));
				} catch (e) {
					if (!(e instanceof RequestFailed) || (e.status !== 404 && e.status !== 422)) {
						this.stalled = true;
						return;
					}
					if (!(e.status === 404 && method === 'DELETE')) this.refused = e.error.message;
				}
				if (this.run?.id !== runId) return;
				this.stalled = false;
				this.writes = this.writes.filter((x) => x.seq !== seq);
				this.#store();
			}
		} finally {
			this.#flushing = false;
		}
	}
}

export const openRun = new OpenRun();

addEventListener('online', () => void openRun.flush());
document.addEventListener('visibilitychange', () => {
	if (document.visibilityState === 'visible') void openRun.flush();
});

export type StartBody = { scheduled: string } | { template: string } | { name: string };

// startRun is never retried: a second POST would start a second run.
export async function startRun(body: StartBody): Promise<Run> {
	const run = await request<Run>('POST', '/api/v1/runs', body);
	openRun.run = run;
	openRun.writes = [];
	return run;
}
