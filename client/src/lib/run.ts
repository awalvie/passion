import { formatDuration } from './exercise';
import { localToday } from './plan';
import type { Section as PlanSection, Step } from './template';

// The shapes of server/api/run.go's responses.
export type Run = {
	id: string;
	template: string | null;
	scheduled: string | null;
	name: string;
	plan: PlanSection[] | null;
	sections: RunSection[];
	// Kept as the server wrote it and sent back as is.
	started_at: string;
	timezone: string;
	local_date: string;
	finished_at: string | null;
	elapsed_seconds: number | null;
	place: string | null;
	notes: string | null;
	sets: LoggedSet[];
	climbs: Climb[];
} & Journal;

export type Journal = {
	sleep: number | null;
	energy: number | null;
	rpe: number | null;
	focus: string | null;
	setting: string | null;
	went_well: string | null;
	next_focus: string | null;
};

export type RunSection = { name: string; notes: string | null; items: RunItem[] };

export type RunItem = { step: RunStep; choice?: never } | { choice: RunChoice; step?: never };

export type RunChoice = {
	id: string;
	name: string;
	notes: string | null;
	pick: number;
	options: Step[];
};

export type StepStatus = 'done' | 'skipped';

export type RunStep = Step & {
	id: string;
	status: StepStatus | null;
	run_notes: string | null;
	elapsed_seconds: number | null;
	from_choice?: RunChoice;
};

// rep_seconds holds each rep's time when they differ, as a ladder's, and
// seconds is then null.
export type SetFields = {
	reps: number | null;
	seconds: number | null;
	rep_seconds?: number[] | null;
	weight_kg: number | null;
};

export type LoggedSet = SetFields & { step: string; number: number; exercise: string };

// cleanSet keeps a typed number inside what server/db/run_set.go accepts. A
// refused set write would be resent with every later set, and refused again.
export function cleanSet(s: SetFields): SetFields {
	const count = (n: number | null) => (n === null ? null : Math.min(1e6, Math.max(0, Math.round(n))));
	const weight = s.weight_kg === null ? null : Math.round(Math.min(9999, Math.max(-9999, s.weight_kg)) * 100) / 100;
	const reps = s.rep_seconds?.length ? s.rep_seconds.map((n) => count(n)!) : null;
	return { reps: count(s.reps), seconds: reps ? null : count(s.seconds), rep_seconds: reps, weight_kg: weight };
}

export type Discipline = 'boulder' | 'sport' | 'trad';
// One go on a climb.
export type Go = 'attempt' | 'send';

export type ClimbFields = {
	step: string;
	position: number;
	discipline: Discipline;
	setting: 'indoor' | 'outdoor';
	board: string | null;
	rope_style: string | null;
	grade: string | null;
	grade_system: string | null;
	goes: Go[];
	seconds: number | null;
	stars: number | null;
	focus: string | null;
	notes: string | null;
};

export type Climb = ClimbFields & { id: string; exercise: string; sent: boolean };

export const disciplines: Record<Discipline, string> = { boulder: 'Boulder', sport: 'Sport', trad: 'Trad' };
// Where a climb was, kept apart from the journal's free-text setting.
export const settings = { indoor: 'Indoor', outdoor: 'Outdoor' } as const;
export const ropes = { lead: 'Lead', top_rope: 'Top rope', auto_belay: 'Auto belay', auto_belay_rope: 'Auto belay with rope', follow: 'Follow' } as const;
export const boards = { none: 'None', kilter: 'Kilter', moon: 'Moon', tension: 'Tension', spray: 'Spray', custom: 'Other' } as const;

// climbWord says how a climb went. An ungraded climb is never a send (rule
// 39), so it counts its goes.
export function climbWord(c: Pick<Climb, 'goes' | 'grade_system'>) {
	const n = c.goes.filter((g) => g === 'send').length;
	if (!c.grade_system) return `${c.goes.length} ${c.goes.length === 1 ? 'go' : 'goes'}`;
	return n > 1 ? `${n} sends` : n ? (c.goes.length === 1 ? 'Flash' : 'Sent') : 'Not sent';
}

export type RunSummary = {
	id: string;
	template: string | null;
	name: string;
	local_date: string;
	started_at: string;
	timezone: string;
	finished_at: string | null;
	elapsed_seconds: number | null;
	place: string | null;
	// Missing from the runs kept on the phone.
	has_journal?: boolean;
};

// The shape of server/api/history.go's response.
export type HistorySession = {
	run: string;
	name: string;
	local_date: string;
	sets: LoggedSet[];
	climbs: Climb[];
};

// A body PUT replaces the whole run, so it sends every field the server keeps.
export function toRunBody(r: Run) {
	return {
		name: r.name,
		sections: r.sections,
		started_at: r.started_at,
		local_date: r.local_date,
		elapsed_seconds: r.elapsed_seconds,
		place: r.place,
		notes: r.notes,
		sleep: r.sleep,
		energy: r.energy,
		rpe: r.rpe,
		focus: r.focus,
		setting: r.setting,
		went_well: r.went_well,
		next_focus: r.next_focus
	};
}

export function stepsOf(r: Pick<Run, 'sections'>): RunStep[] {
	return r.sections.flatMap((s) => s.items.flatMap((i) => (i.step ? [i.step] : [])));
}

// The server marks a step done at its first set, so only the time written
// when the person moves on says that they finished it.
export function isFinished(s: RunStep): boolean {
	return s.status === 'skipped' || s.elapsed_seconds !== null;
}

// A stop is where the player goes next: an unfinished step, or a choice that
// still needs a pick. The player must not pass a choice that asks for one.
function isStop(i: RunItem): boolean {
	return i.choice ? i.choice.pick > 0 : !isFinished(i.step);
}

export function currentStop(r: Pick<Run, 'sections'>): RunItem | undefined {
	return r.sections.flatMap((s) => s.items).find(isStop);
}

// nextStop is the first stop after this step, or else the first one before it.
export function nextStop(r: Pick<Run, 'sections'>, after: string): RunItem | undefined {
	const items = r.sections.flatMap((s) => s.items);
	const i = items.findIndex((it) => it.step?.id === after);
	return [...items.slice(i + 1), ...items.slice(0, i)].find(isStop);
}

export function stopHref(run: string, stop: RunItem | undefined): string {
	if (!stop) return `/run/${run}`;
	return stop.step ? `/run/${run}/step/${stop.step.id}` : `/run/${run}#pick-${stop.choice.id}`;
}

export type SectionMark = 'done' | 'skipped' | 'now' | 'todo';

// sectionMarks marks the section at `at` as now, a section whose items are
// all skipped as skipped, and one whose items are all finished as done.
export function sectionMarks(r: Pick<Run, 'sections'>, at: number): SectionMark[] {
	return r.sections.map((s, i) => {
		if (i === at) return 'now';
		if (!s.items.length || !s.items.every((it) => it.step && isFinished(it.step))) return 'todo';
		return s.items.every((it) => it.step!.status === 'skipped') ? 'skipped' : 'done';
	});
}

// sectionShare is how much of a section is finished, from 0 to 1.
export function sectionShare(s: RunSection): number {
	const done = s.items.filter((it) => it.step && isFinished(it.step)).length;
	return s.items.length ? done / s.items.length : 0;
}

export function setsOf(r: Pick<Run, 'sets'>, step: string): LoggedSet[] {
	return r.sets.filter((s) => s.step === step).sort((a, b) => a.number - b.number);
}

export function climbsOf(r: Pick<Run, 'climbs'>, step: string): Climb[] {
	return r.climbs.filter((c) => c.step === step).sort((a, b) => a.position - b.position);
}

// stepTime is the time a past step shows: what the person logged, exactly,
// else how long the step ran, to the minute. A set exercise keeps its times
// on its sets.
export function stepTime(r: Pick<Run, 'sets' | 'climbs'>, s: RunStep): string | null {
	if (s.status !== 'done' || (s.kind !== 'open' && s.kind !== 'climbing')) return null;
	const logged = (s.kind === 'open' ? setsOf(r, s.id) : climbsOf(r, s.id)).flatMap((x) => (x.seconds ? [x.seconds] : []));
	if (logged.length) return formatDuration(logged.reduce((a, b) => a + b));
	if (s.elapsed_seconds === null) return null;
	return formatDuration(s.elapsed_seconds < 60 ? s.elapsed_seconds : Math.round(s.elapsed_seconds / 60) * 60);
}

// secondsSince is the session clock: the time from the start to now.
export function secondsSince(startedAt: string, now = Date.now()): number {
	return Math.max(0, Math.floor((now - Date.parse(startedAt)) / 1000));
}

// isStale says an unfinished run was left open. It runs until 04:00 after its
// day, so a session that goes past midnight still is one.
export function isStale(r: Pick<Run, 'finished_at' | 'local_date' | 'timezone'>, now = new Date()): boolean {
	return !r.finished_at && r.local_date < localToday(r.timezone, new Date(now.getTime() - 4 * 3600_000));
}
