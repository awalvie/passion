import type { IconName } from './Icon.svelte';

// The shape of server/api/exercise.go's exerciseResponse.
export type Exercise = {
	id: string;
	shipped: boolean;
	name: string;
	kind: Kind;
	notes: string | null;
	source: string | null;
	tags: string[];
	sets: number | null;
	reps: number | null;
	set_rest_seconds: number | null;
	rep_seconds: number | null;
	rep_rest_seconds: number | null;
	prep_seconds: number | null;
	duration_seconds: number | null;
	media: Media[];
	retired_at: string | null;
};

export type Media = { url: string | null; thumb_url: string | null };

export type Kind = 'reps_and_sets' | 'timed_reps' | 'climbing' | 'open';

export type Count =
	| 'sets'
	| 'reps'
	| 'rep_seconds'
	| 'rep_rest_seconds'
	| 'set_rest_seconds'
	| 'prep_seconds'
	| 'duration_seconds';

export const countLabels: Record<Count, string> = {
	sets: 'Sets',
	reps: 'Reps',
	rep_seconds: 'Rep time (s)',
	rep_rest_seconds: 'Rep rest (s)',
	set_rest_seconds: 'Set rest (s)',
	prep_seconds: 'Prep time (s)',
	duration_seconds: 'Duration'
};

export const allCounts = Object.keys(countLabels) as Count[];

// counts are the numbers each player screen uses, in the order the form shows them.
export const kinds: { kind: Kind; label: string; icon: IconName; hint: string; counts: Count[] }[] = [
	{
		kind: 'reps_and_sets',
		label: 'Reps & sets',
		icon: 'list-checks',
		hint: 'Track sets and reps. You move on to the next set by hand.',
		counts: ['sets', 'reps', 'set_rest_seconds']
	},
	{
		kind: 'timed_reps',
		label: 'Timed reps',
		icon: 'timer',
		hint: 'Each rep runs a countdown timer — good for holds, stretches, or timed intervals.',
		counts: ['sets', 'reps', 'rep_seconds', 'rep_rest_seconds', 'set_rest_seconds', 'prep_seconds']
	},
	{
		kind: 'climbing',
		label: 'Climbing',
		icon: 'mountain',
		hint: 'Log each climb with its grade and style.',
		counts: []
	},
	{
		kind: 'open',
		label: 'Open',
		icon: 'layers',
		hint: 'A stopwatch or countdown for an open block.',
		counts: ['duration_seconds']
	}
];

export function kindOf(kind: Kind) {
	return kinds.find((k) => k.kind === kind)!;
}

// A Draft is an exercise as the form holds it: text boxes hold strings, never null.
export type Draft = Pick<Exercise, 'name' | 'kind' | Count> & {
	notes: string;
	source: string;
	tags: string;
	media: { url: string; thumb_url: string }[];
};

export function toDraft(e?: Exercise): Draft {
	return {
		name: e?.name ?? '',
		kind: e?.kind ?? 'reps_and_sets',
		notes: e?.notes ?? '',
		source: e?.source ?? '',
		tags: e?.tags.join(', ') ?? '',
		sets: e?.sets ?? null,
		reps: e?.reps ?? null,
		rep_seconds: e?.rep_seconds ?? null,
		rep_rest_seconds: e?.rep_rest_seconds ?? null,
		set_rest_seconds: e?.set_rest_seconds ?? null,
		prep_seconds: e?.prep_seconds ?? null,
		duration_seconds: e?.duration_seconds ?? null,
		// A new exercise starts with one empty row, as V1's form did.
		media: (e?.media ?? [{ url: null, thumb_url: null }]).map((m) => ({
			url: m.url ?? '',
			thumb_url: m.thumb_url ?? ''
		}))
	};
}

// toBody sends a count only when the form shows it, so switching the type
// drops the numbers the old type used. The server trims the text and drops
// empty media rows.
export function toBody(d: Draft, shown: (c: Count) => boolean) {
	const counts = Object.fromEntries(allCounts.map((c) => [c, shown(c) ? d[c] : null]));
	return {
		name: d.name,
		kind: d.kind,
		notes: d.notes,
		source: d.source,
		tags: d.tags.split(','),
		...counts,
		media: d.media
	};
}

const fieldLabels: Record<string, string> = {
	...countLabels,
	name: 'Name',
	kind: 'Type',
	notes: 'Notes',
	source: 'Source',
	tags: 'Labels',
	body: 'The form'
};

// fieldLabel names a field the server found a problem with, such as media[0].
export function fieldLabel(field: string): string {
	const media = /^media\[(\d+)\]$/.exec(field);
	if (media) return `Media row ${Number(media[1]) + 1}`;
	return fieldLabels[field] ?? field;
}

export function distinct(values: string[]) {
	return [...new Set(values)].sort((a, b) => a.localeCompare(b));
}

export function sourcesOf(exercises: Exercise[]) {
	return distinct(exercises.flatMap((e) => (e.source ? [e.source] : [])));
}

export function summary(d: Pick<Exercise, 'kind' | Count>): string {
	if (d.kind === 'open') return d.duration_seconds ? formatDuration(d.duration_seconds) : '';
	if (d.sets && d.reps) return `${d.sets}×${d.reps}`;
	if (d.sets) return `${d.sets} sets`;
	if (d.reps) return `${d.reps} reps`;
	return '';
}

export function formatDuration(seconds: number): string {
	const parts = [
		[Math.floor(seconds / 3600), 'h'],
		[Math.floor((seconds % 3600) / 60), 'm'],
		[seconds % 60, 's']
	].filter(([n]) => n);
	return parts.map(([n, unit]) => `${n}${unit}`).join(' ') || '0s';
}
