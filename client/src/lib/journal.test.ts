import { expect, test } from 'vitest';
import { cameFrom, excerpt, pieces, pinnedOf, rows, sessionsOn, type JournalEntry, type SessionJournal } from './journal';
import type { RunSummary } from './run';

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

test("a day's sessions come in the order they started, each with its journal", () => {
	const run = (id: string, local_date: string, started_at: string): RunSummary => ({
		id,
		template: null,
		name: 'Strength',
		local_date,
		started_at,
		timezone: 'UTC',
		finished_at: null,
		elapsed_seconds: 3600,
		place: null
	});
	const runs = [run('late', '2026-10-28', '2026-10-28T18:00:00Z'), run('other', '2026-10-27', '2026-10-27T09:00:00Z'), run('early', '2026-10-28', '2026-10-28T07:00:00Z')];
	const day = sessionsOn('2026-10-28', runs, [session('late', '2026-10-28', { rpe: 7 })]);
	expect(day.map((d) => d.run.id)).toEqual(['early', 'late']);
	expect(day[1].journal?.rpe).toBe(7);
	expect(day[0].journal).toBeUndefined();
	expect(sessionsOn('2026-10-28', null, [])).toEqual([]);
});

test('a search of only spaces is no search: pinned entries stay on their card', () => {
	const j = { entries: [entry('a', '2026-10-24', 'Pinned', '2026-10-25T08:00:00Z'), entry('b', '2026-10-20', 'Plain')], sessions: [] };
	expect(rows(j, '   ').map((r) => r.entry?.id)).toEqual(['b']);
});

test('pinnedOf leaves the list it was given as it was', () => {
	const list = [entry('a', '2026-10-24', 'A', '2026-10-20T08:00:00Z'), entry('b', '2026-10-24', 'B'), entry('c', '2026-10-24', 'C', '2026-10-25T08:00:00Z')];
	pinnedOf(list);
	expect(list.map((e) => e.id)).toEqual(['a', 'b', 'c']);
	expect(pinnedOf([])).toEqual([]);
});

// The row shows two lines, so a match 50 characters in is where it can fall off.
test('excerpt cuts only from a match 50 characters in, and keeps the match near the start', () => {
	const words = (n: number) => 'abcd '.repeat(n);
	const at49 = `${words(9)}abcdhips`;
	expect(at49.indexOf('hips')).toBe(49);
	expect(excerpt(at49, 'hips')).toBe(at49);

	const at50 = `${words(10)}hips and then more`;
	expect(at50.indexOf('hips')).toBe(50);
	const cut = excerpt(at50, 'hips');
	expect(cut.startsWith('…')).toBe(true);
	expect(cut.indexOf('hips')).toBeLessThanOrEqual(30);
	expect(cut.endsWith('hips and then more')).toBe(true);
});

test('excerpt shows the whole text when there is nothing to find', () => {
	const long = 'word '.repeat(40);
	expect(excerpt(long, '')).toBe(long);
	expect(excerpt(long, '   ')).toBe(long);
	expect(excerpt(long, 'absent')).toBe(long);
});

test('excerpt brings a match into view when there is no space before it', () => {
	const text = `${'a'.repeat(80)} hips`;
	expect(excerpt(text, 'hips').length).toBeLessThan(text.length);
});

test('sessions of a run kept on the phone, which has no has_journal, still list with their journal', () => {
	const stored = { id: 'r', template: null, name: 'Strength', local_date: '2026-10-28', started_at: '2026-10-28T07:00:00Z', timezone: 'UTC', finished_at: null, elapsed_seconds: null, place: null } as RunSummary;
	const day = sessionsOn('2026-10-28', [stored], [session('r', '2026-10-28', { notes: 'Tired' }), session('elsewhere', '2026-10-28', { notes: 'Other' })]);
	expect(day).toHaveLength(1);
	expect(day[0].journal?.notes).toBe('Tired');
	expect(sessionsOn('2026-10-29', [stored], [])).toEqual([]);
});

test('sessionsOn leaves the runs in the order they came', () => {
	const run = (id: string, started_at: string) => ({ id, local_date: '2026-10-28', started_at }) as RunSummary;
	const runs = [run('late', '2026-10-28T18:00:00Z'), run('early', '2026-10-28T07:00:00Z')];
	sessionsOn('2026-10-28', runs, []);
	expect(runs.map((r) => r.id)).toEqual(['late', 'early']);
});
