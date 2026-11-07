import { beforeEach, expect, test, vi } from 'vitest';

// Items live as own properties, as in a browser.
class MemoryStorage {
	[k: string]: unknown;
	getItem(k: string): string | null {
		return Object.hasOwn(this, k) ? String(this[k]) : null;
	}
	setItem(k: string, v: string) {
		this[k] = String(v);
	}
	removeItem(k: string) {
		delete this[k];
	}
}

let log: string[];
let listeners: Record<string, (e?: { pointerType: string }) => void>;
let oscillators: { frequency: { value: number }; started: boolean }[];
let contexts: FakeAudioContext[];
let stuck: boolean;

class FakeAudioContext {
	state = 'suspended';
	currentTime = 0;
	destination = {};
	sampleRate = 44100;
	constructor() {
		log.push('context');
		contexts.push(this);
	}
	// A stuck resume never settles, as WebKit's does when it will not start.
	resume() {
		log.push('resume');
		if (stuck) return new Promise<void>(() => {});
		this.state = 'running';
		return Promise.resolve();
	}
	suspend() {
		log.push('suspend');
		this.state = 'suspended';
		return Promise.resolve();
	}
	close() {
		log.push('close');
		this.state = 'closed';
		return Promise.resolve();
	}
	createOscillator() {
		const o = { frequency: { value: 0 }, started: false, connect: (n: unknown) => n, start() { o.started = true; }, stop() {} };
		oscillators.push(o);
		return o;
	}
	createBuffer() {
		return {};
	}
	createBufferSource() {
		return { buffer: null, connect: (n: unknown) => n, start: () => log.push('quiet') };
	}
	createGain() {
		return { gain: { setValueAtTime() {}, exponentialRampToValueAtTime() {} }, connect: (n: unknown) => n };
	}
}

beforeEach(() => {
	vi.unstubAllGlobals();
	log = [];
	listeners = {};
	oscillators = [];
	contexts = [];
	stuck = false;
	vi.stubGlobal('localStorage', new MemoryStorage());
	vi.stubGlobal('addEventListener', (name: string, fn: (e?: { pointerType: string }) => void) => (listeners[name] = fn));
	vi.stubGlobal('removeEventListener', (name: string) => delete listeners[name]);
	vi.stubGlobal('document', {
		visibilityState: 'visible' as string,
		addEventListener: (name: string, fn: () => void) => (listeners[`document:${name}`] = fn),
		removeEventListener: (name: string) => delete listeners[`document:${name}`]
	});
	vi.stubGlobal('AudioContext', FakeAudioContext);

	let type = 'auto';
	vi.stubGlobal('navigator', {
		audioSession: {
			get type() {
				return type;
			},
			set type(v: string) {
				log.push(`session ${v}`);
				type = v;
			}
		}
	});
});

const tap = () => listeners.pointerup?.({ pointerType: 'touch' });

// A fresh import is a reload: no audio context yet. Most tests are on a run,
// which listens for taps.
async function load(run = true) {
	vi.resetModules();
	const audio = await import('./audio');
	return { ...audio, leave: run ? audio.listen('r1') : () => {} };
}

test('sound is on until it is switched off, and the choice is kept', async () => {
	const { setSound, soundOn } = await load();
	expect(soundOn()).toBe(true);
	setSound(false);
	expect(soundOn()).toBe(false);
	expect(localStorage.getItem('passion-sound')).toBe('off');
	setSound(true);
	expect(soundOn()).toBe(true);
	expect(localStorage.getItem('passion-sound')).toBeNull();
});

test('the first tap sets playback, to sound on silent, before the audio context starts, and does it once', async () => {
	await load();
	tap();
	tap();
	expect(log).toEqual(['session playback', 'context', 'quiet', 'resume']);
});

// A touch counts as a tap for audio only as it lifts; a mouse as it presses.
test('a touch unlocks as it lifts, and a mouse as it presses', async () => {
	await load();
	listeners.pointerdown({ pointerType: 'touch' });
	listeners.pointerup({ pointerType: 'mouse' });
	expect(contexts).toHaveLength(0);
	listeners.pointerdown({ pointerType: 'mouse' });
	expect(contexts).toHaveLength(1);
});

test('a tone plays only after a tap has unlocked the audio', async () => {
	const { tone } = await load();
	tone(880, 100);
	expect(oscillators).toHaveLength(0);
	tap();
	tone(880, 100);
	expect(oscillators).toHaveLength(1);
	expect(oscillators[0].frequency.value).toBe(880);
	expect(oscillators[0].started).toBe(true);
});

test('a tone is silent while sound is off', async () => {
	const { setSound, tone } = await load();
	tap();
	setSound(false);
	tone(880, 100);
	expect(oscillators).toHaveLength(0);
});

const show = (state: 'hidden' | 'visible') => {
	(document as { visibilityState: string }).visibilityState = state;
	listeners['document:visibilitychange']();
};
const flush = () => new Promise((r) => setTimeout(r, 0));

test('hiding the app suspends the context, and coming back resumes the same one', async () => {
	await load();
	tap();
	log.length = 0;
	show('hidden');
	expect(log).toEqual(['suspend']);
	show('visible');
	expect(log).toEqual(['suspend', 'resume']);
	expect(contexts).toHaveLength(1);
	expect(contexts[0].state).toBe('running');
});

// The sound log showed it: after the app was hidden, the context still said
// running and played nothing, and resume() on a running context does nothing.
test('a context that still says running on return is suspended, then resumed', async () => {
	await load();
	tap();
	show('hidden');
	contexts[0].state = 'running';
	log.length = 0;
	show('visible');
	await flush();
	expect(log).toEqual(['suspend', 'resume']);
	expect(contexts).toHaveLength(1);
});

// The sound log showed it: a context made in a tap after return often never
// started. The one a tap started before may resume, even with no tap.
test('a tap after return resumes the run’s context, and never makes a new one', async () => {
	await load();
	tap();
	show('hidden');
	stuck = true;
	show('visible');
	expect(contexts[0].state).toBe('suspended');
	log.length = 0;
	tap();
	tap();
	expect(log).toEqual(['resume', 'resume']);
	stuck = false;
	tap();
	expect(contexts[0].state).toBe('running');
	expect(contexts).toHaveLength(1);
});

test('a tap that finds the audio stopped resumes it', async () => {
	await load();
	tap();
	contexts[0].state = 'interrupted';
	log.length = 0;
	tap();
	expect(log).toEqual(['resume']);
	expect(contexts).toHaveLength(1);
});

test('a tone resumes a stopped context', async () => {
	const { tone } = await load();
	tap();
	contexts[0].state = 'suspended';
	log.length = 0;
	tone(784, 80);
	expect(log).toEqual(['resume']);
	expect(oscillators).toHaveLength(1);
});

// It would wait in the suspended context and sound late, on return.
test('no tone is made while the app is hidden', async () => {
	const { tone } = await load();
	tap();
	show('hidden');
	log.length = 0;
	tone(784, 80);
	expect(log).toEqual([]);
	expect(oscillators).toHaveLength(0);
});

// With the sound off no tone will ever play, so a tap should not take the
// audio session.
test('a tap does not take the audio session while sound is off', async () => {
	const { setSound } = await load();
	setSound(false);
	tap();
	expect(log).not.toContain('session playback');
});

// The tap on the switch comes before the switch turns the sound on, so the
// switch unlocks the audio itself.
test('switching the sound back on unlocks the audio and beeps', async () => {
	const { setSound } = await load();
	setSound(false);
	tap();
	setSound(true);
	expect(log).toEqual(['session playback', 'context', 'quiet', 'resume']);
	expect(oscillators.map((o) => o.frequency.value)).toEqual([1046]);
});

// Only a run may take the audio.
test('a tap outside a run leaves the audio alone', async () => {
	await load(false);
	tap();
	expect(log).toEqual([]);
	expect(listeners.pointerup).toBeUndefined();
});

test('leaving a run’s pages suspends its audio and stops listening', async () => {
	const { leave } = await load();
	tap();
	log.length = 0;
	leave();
	expect(log).toEqual(['suspend']);
	log.length = 0;
	tap();
	listeners['document:visibilitychange']?.();
	expect(log).toEqual([]);
});

// Going to Today and back to the timer mid-run: the next tone must sound
// with no tap, and a new context would need one.
test('coming back to the same run resumes its context, with no tap', async () => {
	const { leave, listen, tone } = await load();
	tap();
	leave();
	log.length = 0;
	listen('r1');
	expect(log).toEqual(['resume']);
	expect(contexts).toHaveLength(1);
	expect(contexts[0].state).toBe('running');
	tone(784, 80);
	expect(oscillators).toHaveLength(1);
});

test('another run closes the last run’s audio and waits for a tap', async () => {
	const { leave, listen } = await load();
	tap();
	leave();
	log.length = 0;
	listen('r2');
	expect(log).toEqual(['close', 'session auto']);
	tap();
	expect(contexts).toHaveLength(2);
});

test('switching the sound on outside a run beeps, then hands the audio back', async () => {
	vi.useFakeTimers();
	const { setSound } = await load(false);
	setSound(true);
	expect(oscillators.map((o) => o.frequency.value)).toEqual([1046]);
	log.length = 0;
	vi.advanceTimersByTime(400);
	expect(log).toEqual(['close', 'session auto']);
	vi.useRealTimers();
});

// Profile is outside the run. Its beep must not close the run's context, or
// the timer would need a tap on return.
test('switching the sound on mid-run, away from the timer, keeps the run’s audio', async () => {
	vi.useFakeTimers();
	const { leave, setSound, listen } = await load();
	tap();
	leave();
	setSound(true);
	log.length = 0;
	vi.advanceTimersByTime(400);
	expect(log).toEqual(['suspend']);
	listen('r1');
	expect(contexts).toHaveLength(1);
	expect(contexts[0].state).toBe('running');
	vi.useRealTimers();
});

test('leaving the run during a return’s resume leaves the context suspended', async () => {
	const { leave } = await load();
	tap();
	show('hidden');
	contexts[0].state = 'running';
	show('visible');
	leave();
	await flush();
	expect(contexts[0].state).toBe('suspended');
});

test('a finished run hands the audio back', async () => {
	const { release } = await load();
	tap();
	log.length = 0;
	release();
	expect(log).toEqual(['close', 'session auto']);
});
