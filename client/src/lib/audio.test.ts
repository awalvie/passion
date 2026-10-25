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
let listeners: Record<string, () => void>;
let oscillators: { frequency: { value: number }; started: boolean }[];
let contexts: FakeAudioContext[];

class FakeAudioContext {
	state = 'suspended';
	currentTime = 0;
	destination = {};
	constructor() {
		log.push('context');
		contexts.push(this);
	}
	resume() {
		log.push('resume');
		this.state = 'running';
		return Promise.resolve();
	}
	createOscillator() {
		const o = { frequency: { value: 0 }, started: false, connect: (n: unknown) => n, start() { o.started = true; }, stop() {} };
		oscillators.push(o);
		return o;
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
	vi.stubGlobal('localStorage', new MemoryStorage());
	vi.stubGlobal('addEventListener', (name: string, fn: () => void) => (listeners[name] = fn));
	vi.stubGlobal('document', { visibilityState: 'visible', addEventListener: (name: string, fn: () => void) => (listeners[`document:${name}`] = fn) });
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

// A fresh import is a reload: no audio context yet.
async function load() {
	vi.resetModules();
	return import('./audio');
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

test('the first tap sets playback before the audio context starts, and does it once', async () => {
	await load();
	listeners.pointerdown();
	listeners.pointerdown();
	expect(log).toEqual(['session playback', 'context', 'resume', 'resume']);
});

test('a tone plays only after a tap has unlocked the audio', async () => {
	const { tone } = await load();
	tone(880, 100);
	expect(oscillators).toHaveLength(0);
	listeners.pointerdown();
	tone(880, 100);
	expect(oscillators).toHaveLength(1);
	expect(oscillators[0].frequency.value).toBe(880);
	expect(oscillators[0].started).toBe(true);
});

test('a tone is silent while sound is off', async () => {
	const { setSound, tone } = await load();
	listeners.pointerdown();
	setSound(false);
	tone(880, 100);
	expect(oscillators).toHaveLength(0);
});

test('coming back to the app resumes a suspended context, and leaves a running one alone', async () => {
	await load();
	listeners.pointerdown();
	log.length = 0;
	listeners['document:visibilitychange']();
	expect(log).toEqual([]);
	contexts[0].state = 'suspended';
	listeners['document:visibilitychange']();
	expect(log).toEqual(['resume']);
});

// Playback stops music from other apps. With the sound off no tone will ever
// play, so a tap anywhere in the app should not take the audio session.
test('a tap does not take the audio session while sound is off', async () => {
	const { setSound } = await load();
	setSound(false);
	listeners.pointerdown();
	expect(log).not.toContain('session playback');
});

// The tap on the switch comes before the switch turns the sound on, so the
// switch unlocks the audio itself.
test('switching the sound back on unlocks the audio', async () => {
	const { setSound, tone } = await load();
	setSound(false);
	listeners.pointerdown();
	setSound(true);
	tone(880, 100);
	expect(log).toEqual(['session playback', 'context', 'resume']);
	expect(oscillators).toHaveLength(1);
});
