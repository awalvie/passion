import { request } from './api';
import type { Run } from './run';

// The run the person has open. The session screen and the player share it, so
// a set logged on one screen shows on the other without a fetch.
class OpenRun {
	run = $state<Run | null>(null);

	async load(id: string) {
		this.run = await request<Run>('GET', `/api/v1/runs/${id}`);
	}
}

export const openRun = new OpenRun();
