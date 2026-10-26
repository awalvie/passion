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
import { pendingPrefix, timerPrefix } from './session';
import { toStep, type Step } from './template';
import { elapsed, rows, timedRows, timeline } from './timeline';
import { readTimers } from './timerStore';

// A write waiting for the server. A newer write to the same url replaces an
// older one, since each carries the whole list or the whole run.
type Write = { seq: number; method: 'PUT' | 'DELETE'; url: string; body?: unknown };

// opened holds when each step was first reached, so a reload keeps its time.
type Stored = { run: Run; writes: Write[]; opened?: Record<string, number>; drafts?: Record<string, ClimbDraft> };

// The climb on a step's card before its first go.
export type ClimbDraft = { id: string; grade: string | null; seconds: number | null; focus: string | null; notes: string | null };

const storageKey = (id: string) => `${pendingPrefix}${id}`;

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
		.filter((k) => k.startsWith(pendingPrefix))
		.flatMap((k) => readStored(k.slice(pendingPrefix.length))?.run ?? [])
		.filter((r) => r.finished_at === null)
		.sort((a, b) => b.started_at.localeCompare(a.started_at));
}

// discardRun deletes a run once the person confirms, with this phone's copy of
// it. It leaves the open run alone, so Today can discard a stale run.
export async function discardRun(run: { id: string; name: string }): Promise<boolean> {
	if (!confirm(`Discard “${run.name}”? Everything logged in it is deleted.`)) return false;
	await request('DELETE', `/api/v1/runs/${run.id}`);
	localStorage.removeItem(storageKey(run.id));
	localStorage.removeItem(`${timerPrefix}${run.id}`);
	return true;
}

// storedRun is this phone's copy of a run, if it holds one.
export function storedRun(id: string): Run | null {
	return readStored(id)?.run ?? null;
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
	// The step the person last had open on this phone.
	here = $state<string | null>(null);
	#seq = 0;
	#flushing = false;
	#openedAt = new Map<string, number>();
	#drafts = new Map<string, ClimbDraft>();

	async load(id: string) {
		if (this.run?.id !== id) {
			this.refused = '';
			this.stalled = false;
		}
		const stored = readStored(id);
		for (const [step, at] of Object.entries(stored?.opened ?? {})) {
			if (!this.#openedAt.has(step)) this.#openedAt.set(step, at);
		}
		for (const [step, d] of Object.entries(stored?.drafts ?? {})) {
			if (!this.#drafts.has(step)) this.#drafts.set(step, d);
		}
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
	// before it drops the run, because the open page reads it. Today can open
	// another running session on the way, which must stay open.
	async discard() {
		const run = this.run!;
		if (!(await discardRun(run))) return;
		await goto('/');
		this.forget(run);
	}

	// forget drops a deleted run with its unsent writes and its clocks. It
	// closes the run only if it is still the one open.
	forget(run = this.run) {
		if (!run) return;
		localStorage.removeItem(storageKey(run.id));
		localStorage.removeItem(`${timerPrefix}${run.id}`);
		if (this.run?.id === run.id) this.use(null);
	}

	// use opens a run with nothing queued, refused or stalled.
	use(run: Run | null) {
		this.run = run;
		this.writes = [];
		this.refused = '';
		this.stalled = false;
		this.here = null;
	}

	// reopen makes a step not done, with no time.
	#reopen(step: RunStep) {
		step.status = null;
		this.#resume(step);
	}

	// resume restarts a finished step's clock from the time it already had, so a
	// return to add a set does not count the time away.
	#resume(step: RunStep) {
		if (step.elapsed_seconds !== null) this.#openedAt.set(step.id, Date.now() - step.elapsed_seconds * 1000);
		step.elapsed_seconds = null;
	}

	// dropSetWrites drops a set write still waiting for a step, which the
	// server would refuse once the step is gone or skipped.
	#dropSetWrites(step: string) {
		this.writes = this.writes.filter((w) => w.url !== `/api/v1/runs/${this.run!.id}/steps/${step}/sets`);
	}

	// choice finds where a choice sits in the run.
	#choice(id: string) {
		for (const section of this.run!.sections) {
			const i = section.items.findIndex((item) => item.choice?.id === id);
			if (i >= 0) return { section, i, choice: section.items[i].choice! };
		}
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

	// unlog keeps only the first sets as done and reopens the step. A step
	// left with no sets is not done any more.
	unlog(step: RunStep, keep: SetFields[]) {
		if (!keep.length) step.status = null;
		this.#resume(step);
		this.saveBody();
		this.setSets(step, keep);
	}

	// saveBody sends the whole run after a change to its sections or fields.
	saveBody() {
		const run = this.run!;
		this.#queue('PUT', `/api/v1/runs/${run.id}`, toRunBody($state.snapshot(run) as Run));
	}

	// opened notes when the person first reached a step on this phone, which is
	// where the time written when they move on starts.
	opened(step: string) {
		this.here = step;
		if (this.#openedAt.has(step)) return;
		this.#openedAt.set(step, Date.now());
		this.#store();
	}

	draft(step: string): ClimbDraft | null {
		return this.#drafts.get(step) ?? null;
	}

	setDraft(step: string, draft: ClimbDraft | null) {
		if (draft) this.#drafts.set(step, draft);
		else this.#drafts.delete(step);
		this.#store();
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
		const sent = fields.grade_system !== null && fields.goes.includes('send');
		const climb: Climb = { ...fields, id, exercise: step.exercise, sent };
		const i = run.climbs.findIndex((c) => c.id === id);
		if (i >= 0) run.climbs[i] = climb;
		else run.climbs.push(climb);
		if (step.status === null) step.status = 'done';
		this.#queue('PUT', `/api/v1/runs/${run.id}/climbs/${id}`, fields);
	}

	// removeClimb replaces an unsent write of the climb, if any. The server
	// then answers 404, which counts as done. A step left with no climbs is
	// open and not done again, as removeSet leaves it.
	removeClimb(step: RunStep, id: string) {
		const run = this.run!;
		run.climbs = run.climbs.filter((c) => c.id !== id);
		if (!run.climbs.some((c) => c.step === step.id)) {
			this.#reopen(step);
			this.saveBody();
		}
		this.#queue('DELETE', `/api/v1/runs/${run.id}/climbs/${id}`);
	}

	// pick puts the chosen options in the choice's place, as new steps that
	// remember the choice they came from.
	pick(choiceId: string, options: Step[]) {
		const at = this.#choice(choiceId);
		if (!at) return;
		const choice = $state.snapshot(at.choice) as RunChoice;
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
		at.section.items.splice(at.i, 1, ...steps);
		this.saveBody();
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
		this.#dropSetWrites(id);
		this.saveBody();
	}

	// moveStep puts a step at `index` in `section`, counted after it is taken
	// out of its place.
	moveStep(id: string, section: number, index: number) {
		const run = this.run!;
		const from = run.sections.find((s) => s.items.some((it) => it.step?.id === id))!;
		const at = from.items.findIndex((it) => it.step?.id === id);
		const [item] = from.items.splice(at, 1);
		run.sections[section].items.splice(index, 0, item);
		this.saveBody();
	}

	// dropStep puts a step before item `before` of `section`, counted while
	// the step is still in its place, as a drag shows it.
	dropStep(id: string, section: number, before: number) {
		const from = this.run!.sections.findIndex((s) => s.items.some((it) => it.step?.id === id));
		const at = this.run!.sections[from].items.findIndex((it) => it.step?.id === id);
		const index = from === section && at < before ? before - 1 : before;
		if (from === section && index === at) return;
		this.moveStep(id, section, index);
	}

	// shiftStep moves a step one place up or down. Past the edge of its
	// section it goes to the end of the section before, or the start of the
	// one after.
	shiftStep(id: string, by: -1 | 1) {
		const sections = this.run!.sections;
		const s = sections.findIndex((x) => x.items.some((it) => it.step?.id === id));
		const at = sections[s].items.findIndex((it) => it.step?.id === id);
		const inside = at + by >= 0 && at + by < sections[s].items.length;
		if (inside) this.moveStep(id, s, at + by);
		else if (by < 0) this.moveStep(id, s - 1, sections[s - 1].items.length);
		else this.moveStep(id, s + 1, 0);
	}

	// logTimer logs the blocks a running timer finished, including those after
	// the person left its step.
	logTimer() {
		const run = this.run!;
		const t = readTimers(run.id).timed;
		const step = t ? this.step(t.step) : undefined;
		if (!t || !step || isFinished(step)) return;
		const phases = timeline(step, t.extra);
		const done = rows(phases, elapsed(t.clock, Date.now()), t.short);
		const logged = setsOf(run, step.id);
		if (done.length <= logged.length) return;
		this.setSets(step, timedRows(step, phases, done, logged, t.weight));
	}

	// unskip brings a skipped step back, with nothing logged.
	unskip(step: RunStep) {
		this.#reopen(step);
		this.saveBody();
	}

	// addSet plans one more set, which reopens a finished step. The set page
	// shows one row for a step planned with none, so it passes its own count.
	addSet(step: RunStep, count = step.sets ?? 0) {
		step.sets = count + 1;
		this.#resume(step);
		this.saveBody();
	}

	// removeSet drops one of a step's sets, and what it logged if it was done.
	removeSet(step: RunStep, i: number, count: number) {
		const sides = step.per_side ? 2 : 1;
		const logged = setsOf(this.run!, step.id);
		const keep = logged.filter((_, k) => Math.floor(k / sides) !== i);
		step.sets = Math.max(0, count - 1);
		if (!keep.length) this.#reopen(step);
		this.saveBody();
		if (keep.length !== logged.length) this.setSets(step, keep);
	}

	// skip drops what the step logged, as the server does, and any set write
	// still waiting, which the server would refuse.
	skip(step: RunStep) {
		const run = this.run!;
		step.status = 'skipped';
		step.elapsed_seconds = this.#elapsed(step.id);
		run.sets = run.sets.filter((s) => s.step !== step.id);
		this.#dropSetWrites(step.id);
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
			const steps = new Set(stepsOf(run).map((s) => s.id));
			const opened = Object.fromEntries([...this.#openedAt].filter(([step]) => steps.has(step)));
			const drafts = Object.fromEntries([...this.#drafts].filter(([step]) => steps.has(step)));
			const stored: Stored = { run: $state.snapshot(run) as Run, writes: $state.snapshot(this.writes), opened, drafts };
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
// iOS can close a backgrounded app without a blur, so a field still being
// typed in is blurred on the way out and its change handler saves it.
document.addEventListener('visibilitychange', () => {
	if (document.visibilityState === 'visible') void openRun.flush();
	else (document.activeElement as HTMLElement | null)?.blur();
});

export type StartBody = { scheduled: string } | { template: string } | { name: string };

// startRun is never retried: a second POST would start a second run.
export async function startRun(body: StartBody): Promise<Run> {
	const run = await request<Run>('POST', '/api/v1/runs', body);
	// The last run's unsent writes stay stored, and go when it opens again.
	openRun.use(run);
	return run;
}
