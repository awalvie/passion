import type { Exercise } from './exercise';

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

