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

export type SetFields = { reps: number | null; seconds: number | null; weight_kg: number | null };

export type LoggedSet = SetFields & { step: string; number: number; exercise: string };

// cleanSet keeps a typed number inside what server/db/run_set.go accepts. A
// refused set write would be resent with every later set, and refused again.
export function cleanSet(s: SetFields): SetFields {
	const count = (n: number | null) => (n === null ? null : Math.min(1e6, Math.max(0, Math.round(n))));
	const weight = s.weight_kg === null ? null : Math.round(Math.min(9999, Math.max(-9999, s.weight_kg)) * 100) / 100;
	return { reps: count(s.reps), seconds: count(s.seconds), weight_kg: weight };
}

export type Discipline = 'boulder' | 'sport' | 'trad';
export type Outcome = 'onsight' | 'flash' | 'redpoint' | 'hangdog' | 'working';

export type ClimbFields = {
	step: string;
	position: number;
	discipline: Discipline;
	setting: 'indoor' | 'outdoor';
	board: string | null;
	rope_style: string | null;
	grade: string | null;
	grade_system: string | null;
	outcome: Outcome | null;
	attempts: number | null;
	seconds: number | null;
	stars: number | null;
	focus: string | null;
	notes: string | null;
};

export type Climb = ClimbFields & { id: string; exercise: string; sent: boolean };

export type RunSummary = {
	id: string;
	template: string | null;
	name: string;
	local_date: string;
	started_at: string;
	finished_at: string | null;
	elapsed_seconds: number | null;
	place: string | null;
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

export function currentStep(r: Pick<Run, 'sections'>): RunStep | undefined {
	return stepsOf(r).find((s) => !isFinished(s));
}

export function setsOf(r: Pick<Run, 'sets'>, step: string): LoggedSet[] {
	return r.sets.filter((s) => s.step === step).sort((a, b) => a.number - b.number);
}

export function climbsOf(r: Pick<Run, 'climbs'>, step: string): Climb[] {
	return r.climbs.filter((c) => c.step === step).sort((a, b) => a.position - b.position);
}

// secondsSince is the session clock: the time from the start to now.
export function secondsSince(startedAt: string, now = Date.now()): number {
	return Math.max(0, Math.floor((now - Date.parse(startedAt)) / 1000));
}
