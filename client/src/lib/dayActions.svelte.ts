import { goto, invalidateAll } from '$app/navigation';
import { describe, request } from './api';
import type { ScheduledDay } from './plan';
import { startRun } from './runState.svelte';

const labels: Record<string, string> = { local_date: 'That day', template: 'That session' };

// Start, move and remove a planned session, for any page that lists a day.
// DaySheets shows the questions a move and a repeat's remove ask.
export class DayActions {
	// What is waiting: a row's move or remove.
	doing = $state<string | null>(null);
	error = $state({ id: '', message: '' });
	starting = $state(false);
	// A repeat's day asks whether its later days go too.
	ending = $state<ScheduledDay | null>(null);
	endOpen = $state(false);
	moving = $state<ScheduledDay | null>(null);
	moveOpen = $state(false);
	moveTo = $state('');

	async start(d: ScheduledDay) {
		this.starting = true;
		this.error = { id: '', message: '' };
		try {
			const run = await startRun({ scheduled: d.id });
			await goto(`/run/${run.id}`);
		} catch (e) {
			this.error = { id: d.id, message: describe(e, (f) => labels[f] ?? f) };
			// The run can start even when the answer is lost; the reload shows it.
			await invalidateAll();
		} finally {
			this.starting = false;
		}
	}

	openMove(d: ScheduledDay) {
		this.moving = d;
		this.moveOpen = true;
		this.moveTo = d.local_date;
		this.error = { id: '', message: '' };
	}

	async move(d: ScheduledDay, to: string) {
		this.doing = `move:${d.id}`;
		this.error = { id: '', message: '' };
		try {
			await request('PUT', `/api/v1/scheduled-sessions/${d.id}`, { local_date: to });
			await invalidateAll();
			this.moveOpen = false;
		} catch (e) {
			this.error = { id: d.id, message: describe(e, (f) => labels[f] ?? f) };
		} finally {
			this.doing = null;
		}
	}

	async remove(d: ScheduledDay, later = false) {
		if (d.repeat && !this.endOpen) {
			this.ending = d;
			this.endOpen = true;
			this.error = { id: '', message: '' };
			return;
		}
		const warning = d.cycle ? ' It comes back if you change the dates, the block or the days of its cycle.' : '';
		if (!d.repeat && !confirm(`Take ${d.template_name} off this day?${warning}`)) return;
		this.doing = `remove:${d.id}`;
		this.error = { id: '', message: '' };
		try {
			if (later) await request('POST', `/api/v1/repeats/${d.repeat}/end`, { from: d.local_date });
			else await request('DELETE', `/api/v1/scheduled-sessions/${d.id}`);
			await invalidateAll();
			this.endOpen = false;
		} catch (e) {
			this.error = { id: d.id, message: describe(e, (f) => labels[f] ?? f) };
		} finally {
			this.doing = null;
		}
	}
}
