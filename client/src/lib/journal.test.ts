import { expect, test } from 'vitest';
import { cameFrom, excerpt, pieces, pinnedOf, rows, type JournalEntry, type SessionJournal } from './journal';

const entry = (id: string, local_date: string, body: string, pinned_at: string | null = null): JournalEntry => ({
	id,
	local_date,
	body,
	pinned_at,
	run: null,
	run_name: null,
	run_elapsed_seconds: null
});

const session = (run: string, local_date: string, notes: Partial<SessionJournal> = {}): SessionJournal => ({
	run,
	name: 'Strength',
	local_date,
	rpe: null,
	went_well: null,
	next_focus: null,
	notes: null,
	...notes
});

test('the newest pin comes first, whatever day the entry is about', () => {
	const old = entry('a', '2026-09-01', 'Old', '2026-10-25T08:00:00Z');
	const fresh = entry('b', '2026-10-24', 'Fresh', '2026-10-20T08:00:00Z');
	expect(pinnedOf([fresh, entry('c', '2026-10-24', 'Not pinned'), old]).map((e) => e.id)).toEqual(['a', 'b']);
});

// The server drops trailing zeros from fractions of a second, so the times
// must be compared as times, not as text.
test('pins a moment apart keep their order', () => {
	const first = entry('a', '2026-10-24', 'First', '2026-10-25T08:00:00.12Z');
	const then = entry('b', '2026-10-24', 'Then', '2026-10-25T08:00:00.123Z');
	expect(pinnedOf([first, then]).map((e) => e.id)).toEqual(['b', 'a']);
});

test('a journal page goes back to Today or a session only when it came from one', () => {
	expect(cameFrom('/').label).toBe('Today');
	expect(cameFrom('/run/abc-1')).toEqual({ href: '/run/abc-1', label: 'Session' });
	for (const from of [null, '/run/', '/run/a/b', '//evil.com', 'https://x']) {
		expect(cameFrom(from)).toEqual({ href: '/journal', label: 'Journal' });
	}
});

test('the list leaves pinned entries to their card, and mixes in session journals by day', () => {
	const j = {
		entries: [entry('a', '2026-10-24', 'Pinned', '2026-10-25T08:00:00Z'), entry('b', '2026-10-20', 'Hips far from the wall')],
		sessions: [session('s', '2026-10-22', { next_focus: 'Rest more' })]
	};
	expect(rows(j, '').map((r) => r.entry?.id ?? r.session?.run)).toEqual(['s', 'b']);
});

test('a search finds pinned entries, and session journals by name or note', () => {
	const j = {
		entries: [entry('a', '2026-10-24', 'Hips in', '2026-10-25T08:00:00Z'), entry('b', '2026-10-20', 'Feet first')],
		sessions: [session('s', '2026-09-28', { notes: 'HIPS far from the wall' }), session('t', '2026-09-27', { went_well: 'Crimps' })]
	};
	expect(rows(j, ' hips ').map((r) => r.entry?.id ?? r.session?.run)).toEqual(['a', 's']);
	expect(rows(j, 'strength').map((r) => r.session?.run)).toEqual(['s', 't']);
});

test('a long text starts near its first match, and a short one in full', () => {
	const long = `${'Warm-up went fine and the board felt good today. '.repeat(3)}Then my hips came off the wall.`;
	expect(excerpt('Hips in.', 'hips')).toBe('Hips in.');
	const cut = excerpt(long, 'hips');
	expect(cut.startsWith('…')).toBe(true);
	expect(cut).toContain('my hips came off');
	expect(cut.length).toBeLessThan(long.length);
});

test('pieces marks every match, in any case', () => {
	expect(pieces('Hips in, hips out', 'hips')).toEqual([
		{ text: 'Hips', hit: true },
		{ text: ' in, ', hit: false },
		{ text: 'hips', hit: true },
		{ text: ' out', hit: false }
	]);
	expect(pieces('Hips in', ' ')).toEqual([{ text: 'Hips in', hit: false }]);
});
