import { goto } from '$app/navigation';
import { RequestFailed, request, unreachable } from './api';
import { newId } from './id';
import {
	cleanSet,
	isFinished,
	setsOf,
	stepsOf,
	toRunBody,
	type Climb,
	type ClimbFields,
	type Run,
	type RunChoice,
	type RunStep,
	type SetFields
} from './run';
import type { Exercise } from './exercise';
import { toStep, type Step } from './template';
import { elapsed, rows, timeline } from './timeline';
import { readTimers } from './timerStore';

// A write waiting for the server. A newer write to the same url replaces an
// older one, since each carries the whole list or the whole run.
type Write = { seq: number; method: 'PUT' | 'DELETE'; url: string; body?: unknown };

type Stored = { run: Run; writes: Write[] };

const storageKey = (id: string) => `passion-pending:${id}`;

function readStored(id: string): Stored | null {
	try {
		const raw = localStorage.getItem(storageKey(id));
		return raw ? (JSON.parse(raw) as Stored) : null;
	} catch {
		localStorage.removeItem(storageKey(id));
		return null;
	}
}

// storedRuns are the unfinished runs this phone holds a copy of, newest first.
export function storedRuns(): Run[] {
	return Object.keys(localStorage)
		.filter((k) => k.startsWith('passion-pending:'))
		.flatMap((k) => readStored(k.slice('passion-pending:'.length))?.run ?? [])
		.filter((r) => r.finished_at === null)
		.sort((a, b) => b.started_at.localeCompare(a.started_at));
}

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
	#openedAt = new Map<string, number>();

	async load(id: string) {
		if (this.run?.id !== id) {
			this.refused = '';
			this.stalled = false;
		}
		const stored = readStored(id);
		const useStored = () => {
			this.run = stored!.run;
			this.writes = stored!.writes;
			this.#seq = Math.max(0, ...stored!.writes.map((w) => w.seq));
			void this.flush();
		};
		// Unsent writes make the phone's copy newer than the server's.
		if (stored?.writes.length) return useStored();
		try {
			this.run = await request<Run>('GET', `/api/v1/runs/${id}`);
			this.writes = [];
			this.#store();
		} catch (e) {
			// With no signal, the copy on the phone still runs the session.
			if (!unreachable(e) || !stored) throw e;
			useStored();
		}
	}

	// discard deletes the run once the person confirms. It leaves for Today
	// before it forgets the run, because the open page reads it.
	async discard() {
		const run = this.run!;
		if (!confirm(`Discard “${run.name}”? Everything logged in it is deleted.`)) return;
		await request('DELETE', `/api/v1/runs/${run.id}`);
		await goto('/');
		this.forget();
	}

	// forget drops a deleted run with its unsent writes and its clocks.
	forget() {
		if (this.run) {
			localStorage.removeItem(storageKey(this.run.id));
			localStorage.removeItem(`passion-timer:${this.run.id}`);
		}
		this.run = null;
		this.writes = [];
		this.refused = '';
		this.stalled = false;
	}

	step(id: string): RunStep | undefined {
		return this.run ? stepsOf(this.run).find((s) => s.id === id) : undefined;
	}

	// setSets replaces a step's sets. The server marks the step done at its
	// first set, and this mirrors that, so the next body write agrees.
	setSets(step: RunStep, typed: SetFields[]) {
		const run = this.run!;
		const sets = typed.map(cleanSet);
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

	// opened notes when the person first reached a step on this phone, which is
	// where the time written when they move on starts.
	opened(step: string) {
		if (!this.#openedAt.has(step)) this.#openedAt.set(step, Date.now());
	}

	#elapsed(step: string) {
		const at = this.#openedAt.get(step);
		return at === undefined ? 0 : Math.round((Date.now() - at) / 1000);
	}

	// finish marks that the person moved on from a step, keeping what it logged.
	finish(step: RunStep) {
		step.elapsed_seconds = this.#elapsed(step.id);
		this.saveBody();
	}

	// putClimb writes a climb under an id this phone picks, so a retry
	// writes the same climb. The server marks the step done at its first climb.
	putClimb(step: RunStep, id: string, fields: ClimbFields) {
		const run = this.run!;
		const sent =
			fields.grade_system !== null && ['onsight', 'flash', 'redpoint'].includes(fields.outcome ?? '');
		const climb: Climb = { ...fields, id, exercise: step.exercise, sent };
		const i = run.climbs.findIndex((c) => c.id === id);
		if (i >= 0) run.climbs[i] = climb;
		else run.climbs.push(climb);
		if (step.status === null) step.status = 'done';
		this.#queue('PUT', `/api/v1/runs/${run.id}/climbs/${id}`, fields);
	}

	// removeClimb replaces an unsent write of the climb, if any. The server
	// then answers 404, which counts as done.
	removeClimb(id: string) {
		const run = this.run!;
		run.climbs = run.climbs.filter((c) => c.id !== id);
		this.#queue('DELETE', `/api/v1/runs/${run.id}/climbs/${id}`);
	}

	// pick puts the chosen options in the choice's place, as new steps that
	// remember the choice they came from.
	pick(choiceId: string, options: Step[]) {
		for (const section of this.run!.sections) {
			const i = section.items.findIndex((item) => item.choice?.id === choiceId);
			if (i < 0) continue;
			const choice = $state.snapshot(section.items[i].choice!) as RunChoice;
			const steps = options.map((o) => ({
				step: {
					...($state.snapshot(o) as Step),
					id: newId(),
					status: null,
					run_notes: null,
					elapsed_seconds: null,
					from_choice: choice
				}
			}));
			section.items.splice(i, 1, ...steps);
			this.saveBody();
			return;
		}
	}

	// addStep puts a library exercise at the end of a section, the last one
	// unless told. An open run starts with no sections, so the first exercise
	// makes one.
	addStep(e: Exercise, section?: number) {
		const run = this.run!;
		if (!run.sections.length) run.sections.push({ name: 'Exercises', notes: null, items: [] });
		const step: RunStep = { ...toStep(e), id: newId(), status: null, run_notes: null, elapsed_seconds: null };
		run.sections[section ?? run.sections.length - 1].items.push({ step });
		this.saveBody();
	}

	addSection(name: string) {
		this.run!.sections.push({ name, notes: null, items: [] });
		this.saveBody();
	}

	// removeStep drops a step nothing was logged against, and any set write
	// still waiting for it, which the server would refuse.
	removeStep(id: string) {
		const run = this.run!;
		for (const section of run.sections) section.items = section.items.filter((it) => it.step?.id !== id);
		this.writes = this.writes.filter((w) => w.url !== `/api/v1/runs/${run.id}/steps/${id}/sets`);
		this.saveBody();
	}

	// removeOption drops one option of a choice not yet picked. The last
	// option takes the choice with it.
	removeOption(choiceId: string, k: number) {
		for (const section of this.run!.sections) {
			const i = section.items.findIndex((item) => item.choice?.id === choiceId);
			if (i < 0) continue;
			const choice = section.items[i].choice!;
			choice.options.splice(k, 1);
			if (!choice.options.length) section.items.splice(i, 1);
			else choice.pick = Math.min(choice.pick, choice.options.length);
			this.saveBody();
			return;
		}
	}

	// logTimer logs the blocks a running timer finished, including those after
	// the person left its step.
	logTimer() {
		const run = this.run!;
		const t = readTimers(run.id).timed;
		const step = t ? this.step(t.step) : undefined;
		if (!t || !step || isFinished(step)) return;
		const done = rows(timeline(step), elapsed(t.clock, Date.now()), t.short);
		if (done.length <= setsOf(run, step.id).length) return;
		this.setSets(
			step,
			done.map((reps) => ({ reps, seconds: step.rep_seconds, weight_kg: t.weight }))
		);
	}

	// unskip brings a skipped step back, with nothing logged.
	unskip(step: RunStep) {
		step.status = null;
		step.elapsed_seconds = null;
		this.saveBody();
	}

	// addSet plans one more set, which reopens a finished step.
	addSet(step: RunStep) {
		step.sets = (step.sets ?? 0) + 1;
		step.elapsed_seconds = null;
		this.saveBody();
	}

	// skip drops what the step logged, as the server does, and any set write
	// still waiting, which the server would refuse.
	skip(step: RunStep) {
		const run = this.run!;
		step.status = 'skipped';
		step.elapsed_seconds = this.#elapsed(step.id);
		run.sets = run.sets.filter((s) => s.step !== step.id);
		this.writes = this.writes.filter((w) => w.url !== `/api/v1/runs/${run.id}/steps/${step.id}/sets`);
		this.saveBody();
	}

	#queue(method: Write['method'], url: string, body?: unknown) {
		this.writes = [...this.writes.filter((w) => w.url !== url), { seq: ++this.#seq, method, url, body }];
		this.#store();
		void this.flush();
	}

	// The phone keeps the open run until it is finished or discarded, so the
	// app reopens without a signal. A full or blocked storage only costs that.
	#store() {
		const run = this.run;
		if (!run) return;
		try {
			const stored: Stored = { run: $state.snapshot(run) as Run, writes: $state.snapshot(this.writes) };
			localStorage.setItem(storageKey(run.id), JSON.stringify(stored));
		} catch {
			/* the queue in memory still sends */
		}
	}

	// settle waits for every queued write, and says whether they all went.
	async settle(): Promise<boolean> {
		this.stalled = false;
		for (let tries = 0; this.writes.length && tries < 50; tries++) {
			await this.flush();
			if (this.stalled) return false;
			if (this.writes.length) await new Promise((r) => setTimeout(r, 200));
		}
		return !this.writes.length;
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
			// Another run opened while a write was in flight, and its load could
			// not start a flush of its own.
			if (this.run && this.run.id !== runId && this.writes.length) void this.flush();
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
	// The last run's unsent writes stay stored, and go when it opens again.
	openRun.run = run;
	openRun.writes = [];
	openRun.refused = '';
	openRun.stalled = false;
	return run;
}
