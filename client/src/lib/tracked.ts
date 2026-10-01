import { request } from './api';
import type { Exercise } from './exercise';
import type { HistorySession } from './run';

// The exercises whose best weight Today shows, newest choice first. The list
// lives on this phone.
const trackedKey = 'passion-tracked';

export function tracked(): string[] {
	return JSON.parse(localStorage.getItem(trackedKey) ?? '[]') as string[];
}

export function track(id: string) {
	localStorage.setItem(trackedKey, JSON.stringify([id, ...tracked().filter((x) => x !== id)]));
}

export type Best = { name: string; kg: number | null };

// bestWeight is the heaviest set of an exercise from one day to another, both
// included.
export async function bestWeight(id: string, from: string, to: string): Promise<Best> {
	const [exercise, history] = await Promise.all([
		request<Exercise>('GET', `/api/v1/exercises/${id}`),
		request<{ sessions: HistorySession[] }>('GET', `/api/v1/exercises/${id}/history`)
	]);
	const kgs = history.sessions
		.filter((s) => from <= s.local_date && s.local_date <= to)
		.flatMap((s) => s.sets.flatMap((set) => (set.weight_kg === null ? [] : [set.weight_kg])));
	return { name: exercise.name, kg: kgs.length ? Math.max(...kgs) : null };
}
