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

export const kinds: { kind: Kind; label: string; icon: IconName }[] = [
	{ kind: 'reps_and_sets', label: 'Reps & sets', icon: 'list-checks' },
	{ kind: 'timed_reps', label: 'Timed reps', icon: 'timer' },
	{ kind: 'climbing', label: 'Climbing', icon: 'mountain' },
	{ kind: 'open', label: 'Open', icon: 'layers' }
];

export function kindOf(kind: Kind) {
	return kinds.find((k) => k.kind === kind)!;
}
