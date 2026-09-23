import { formatDuration, kindOf, summary, type Exercise } from './exercise';

// The shapes of server/api/session_template.go's responses.
export type SessionTemplate = {
	id: string;
	shipped: boolean;
	name: string;
	notes: string | null;
	source: string | null;
	color: string | null;
	needs: string | null;
	tags: string[];
	sections: Section[];
	retired_at: string | null;
};

export type Section = { name: string; notes: string | null; items: Item[] };

export type Item = { step: Step; choice?: never } | { choice: Choice; step?: never };

// A step is a library exercise's id and the session's own copy of its fields.
export type Step = { exercise: string } & Omit<Exercise, 'id' | 'shipped' | 'retired_at'>;

export type Choice = { name: string; notes: string | null; pick: number; options: Step[] };

// stepMeta is the one muted line under a step's name, as V1's preview wrote it.
export function stepMeta(s: Step): string {
	const kind = kindOf(s.kind).label;
	if (s.kind === 'open') {
		return `${kind} · ${s.duration_seconds ? formatDuration(s.duration_seconds) : 'Open-ended'}`;
	}
	return [kind, summary(s), s.rep_seconds ? `${s.rep_seconds}s rep` : ''].filter(Boolean).join(' · ');
}

// pick is the fewest options to do, so 0 makes the whole choice optional.
export function choiceMeta(c: Choice): string {
	const n = c.options.length;
	if (c.pick === 0) return `Optional · any of ${n}`;
	if (c.pick === n) return `Do all ${n}`;
	return `Pick at least ${c.pick} of ${n}`;
}
