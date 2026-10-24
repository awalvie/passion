import { expect, test, vi } from 'vitest';
import { loadTemplates } from './template';

vi.mock('$app/navigation', () => ({ goto: vi.fn() }));

test('loadTemplates reads the template list', async () => {
	const fetch = vi.fn(async () => Response.json({ session_templates: [{ id: 't1' }] }));
	vi.stubGlobal('fetch', fetch);
	vi.stubGlobal('localStorage', { getItem: () => null });
	expect(await loadTemplates()).toStrictEqual([{ id: 't1' }]);
	expect(fetch).toHaveBeenCalledWith('/api/v1/session-templates', expect.objectContaining({ method: 'GET' }));
	vi.unstubAllGlobals();
});
