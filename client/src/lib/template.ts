import { fieldLabel, formatDuration, kindOf, summary, type Exercise } from './exercise';

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

export function moveEntry<T>(list: T[], i: number, by: number) {
	[list[i], list[i + by]] = [list[i + by], list[i]];
}

// A TemplateDraft is a template as the editor holds it: text boxes hold
// strings, and the sections are the editor's own copy.
export type TemplateDraft = {
	name: string;
	notes: string;
	source: string;
	color: string;
	needs: string;
	tags: string;
	sections: Section[];
};

export function toTemplateDraft(t?: SessionTemplate): TemplateDraft {
	return {
		name: t?.name ?? '',
		notes: t?.notes ?? '',
		source: t?.source ?? '',
		color: t?.color ?? '',
		needs: t?.needs ?? '',
		tags: t?.tags.join(', ') ?? '',
		sections: structuredClone(t?.sections ?? [])
	};
}

// The server trims the text and treats an empty box as not set.
export function toTemplateBody(d: TemplateDraft) {
	return { ...d, color: d.color || null, tags: d.tags.split(',') };
}

const templateLabels: Record<string, string> = {
	color: 'Color',
	needs: 'Needs',
	pick: 'Pick',
	options: 'Options',
	exercise: 'Exercise'
};

// templateFieldLabel names a path the server found a problem with, such as
// sections[1].items[2].step.sets, as "Section 2, item 3: Sets".
export function templateFieldLabel(path: string): string {
	const where: string[] = [];
	let rest = path;
	for (const [pattern, word] of [
		[/^sections\[(\d+)\]\.?/, 'Section'],
		[/^items\[(\d+)\]\.?(step\.|choice\.)?/, 'item'],
		[/^options\[(\d+)\]\.?/, 'option']
	] as const) {
		const m = pattern.exec(rest);
		if (!m) continue;
		where.push(`${word} ${Number(m[1]) + 1}`);
		rest = rest.slice(m[0].length);
	}
	const field = rest && (templateLabels[rest] ?? fieldLabel(rest));
	if (!where.length) return field;
	return field ? `${where.join(', ')}: ${field}` : where.join(', ');
}
