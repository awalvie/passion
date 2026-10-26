import { request } from './api';

export type JournalEntry = {
	id: string;
	local_date: string;
	body: string;
	// The session it was written during, with its name and length.
	run: string | null;
	run_name: string | null;
	run_elapsed_seconds: number | null;
	pinned_at: string | null;
};

// A session's own journal, from the form at its end.
export type SessionJournal = {
	run: string;
	name: string;
	local_date: string;
	rpe: number | null;
	went_well: string | null;
	next_focus: string | null;
	notes: string | null;
};

export type Journal = { entries: JournalEntry[]; sessions: SessionJournal[] };

export const loadJournal = () => request<Journal>('GET', '/api/v1/journal');

// The newest pin first, so a fresh pin shows at the top.
export const pinnedOf = (entries: JournalEntry[]) =>
	entries.filter((e) => e.pinned_at).sort((a, b) => Date.parse(b.pinned_at!) - Date.parse(a.pinned_at!));

// A session journal's notes, labelled as History labels them.
export function notesOf(s: SessionJournal): [label: string, text: string][] {
	return (
		[
			['What went well', s.went_well],
			['Next time', s.next_focus],
			['Notes', s.notes]
		] as [string, string | null][]
	).filter((n): n is [string, string] => !!n[1]);
}

export type Row = { date: string; entry?: JournalEntry; session?: SessionJournal };

// rows is the list under the pinned card: entries and session journals, the
// newest day first. A search finds pinned entries too.
export function rows(j: Journal, q: string): Row[] {
	const needle = q.trim().toLowerCase();
	const entries = j.entries.filter((e) => (needle ? e.body.toLowerCase().includes(needle) : !e.pinned_at));
	const sessions = j.sessions.filter(
		(s) => !needle || s.name.toLowerCase().includes(needle) || notesOf(s).some(([, t]) => t.toLowerCase().includes(needle))
	);
	return [
		...entries.map((entry) => ({ date: entry.local_date, entry })),
		...sessions.map((session) => ({ date: session.local_date, session }))
	].sort((a, b) => b.date.localeCompare(a.date));
}

// excerpt starts a long text near the first match, so a three-line row shows
// the word that was searched for.
export function excerpt(text: string, q: string): string {
	const at = q.trim() ? text.toLowerCase().indexOf(q.trim().toLowerCase()) : -1;
	if (at < 100) return text;
	const from = text.lastIndexOf(' ', at - 40);
	return `…${text.slice(from + 1)}`;
}

// pieces splits text around each match, for marking it.
export function pieces(text: string, q: string): { text: string; hit: boolean }[] {
	const needle = q.trim().toLowerCase();
	if (!needle) return [{ text, hit: false }];
	const out: { text: string; hit: boolean }[] = [];
	const lower = text.toLowerCase();
	let i = 0;
	for (let at = lower.indexOf(needle); at >= 0; at = lower.indexOf(needle, i)) {
		if (at > i) out.push({ text: text.slice(i, at), hit: false });
		out.push({ text: text.slice(at, at + needle.length), hit: true });
		i = at + needle.length;
	}
	if (i < text.length) out.push({ text: text.slice(i), hit: false });
	return out;
}
