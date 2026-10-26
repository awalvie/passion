import { expect, test, vi } from 'vitest';
import { choiceMeta, loadTemplates, type Choice, type Step } from './template';

vi.mock('$app/navigation', () => ({ goto: vi.fn() }));

test('loadTemplates reads the template list', async () => {
	const fetch = vi.fn(async () => Response.json({ session_templates: [{ id: 't1' }] }));
	vi.stubGlobal('fetch', fetch);
	vi.stubGlobal('localStorage', { getItem: () => null });
	expect(await loadTemplates()).toStrictEqual([{ id: 't1' }]);
	expect(fetch).toHaveBeenCalledWith('/api/v1/session-templates', expect.objectContaining({ method: 'GET' }));
	vi.unstubAllGlobals();
});

test('choiceMeta says nothing of a choice with one option to do', () => {
	const choice = (pick: number, n: number) => ({ name: 'C', notes: null, pick, options: Array<Step>(n) }) as Choice;
	expect(choiceMeta(choice(1, 1))).toBe('');
	expect(choiceMeta(choice(0, 1))).toBe('Optional · any of 1');
	expect(choiceMeta(choice(2, 2))).toBe('Do all 2');
	expect(choiceMeta(choice(1, 3))).toBe('Pick at least 1 of 3');
});
